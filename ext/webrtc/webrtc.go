package webrtc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/costinm/ugate/pkg/auth"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/pion/sctp"
	"github.com/pion/webrtc/v3"
)

type RTC struct {
	Conn map[string]sctp.Association

	UGate *ugatesvc.UGate
}

// Returns a json with an offer for connecting this host, to allow a web or dmesh client
// to initiate a data channel.
// To reduce RT, the client acts as WebRTC server, includes an offer.
func (rtc *RTC) RTCDirectHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	off, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		w.WriteHeader(501)
		return
	}

	offer := webrtc.SessionDescription{}
	err = json.Unmarshal(off, &offer)
	if err != nil {
		log.Println(err)
		w.WriteHeader(501)
		return
	}
	fmt.Println("IN: ", string(off))
	ans, err := DialWebRTC(&offer)

	if err != nil {
		log.Println(err)
		w.WriteHeader(501)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	ansb, err := json.Marshal(ans)
	if err != nil {
		return
	}
	fmt.Println("OUT: ", string(ansb))
	w.Write(ansb)
}

//
func DialWebRTC(inOffer *webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{}, //"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Register data channel creation handling
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

		// Register channel opening handling
		d.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d'", d.Label(), d.ID())

			for range time.NewTicker(60 * time.Second).C {
				message := "hi"
				fmt.Printf("Sending '%s'\n", message)

				// Send the message as text
				sendErr := d.SendText(message)
				if sendErr != nil {
					log.Println("Send err", sendErr)
				}
			}
		})

		// Register text message handling
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))
		})
	})


	// Set the remote SessionDescription. Starts the transport already !
	err = peerConnection.SetRemoteDescription(*inOffer)
	if err != nil {
		return nil, err
	}

	// After setRemoteDescription - we can use the remote IP as a candidate.
	// It would only work if remote has a public address - for example
	// IPv6.
	// Alternatively, the request could include a known 'gateway'
	//peerConnection.AddICECandidate(webrtc.ICECandidateInit{
	//	Candidate: "candidate:1932572598 2 udp 2130706431 2601:647:6100:449c:b62e:99ff:fead:e42 55174 typ host",
	//})

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Println("OnConnectionStateChange: ", state, peerConnection.SignalingState())
		if state == webrtc.PeerConnectionStateConnected {
			//log.Println(peerConnection.SCTP().Transport().GetRemoteCertificate())
			log.Println(peerConnection.SCTP().Transport().GetLocalParameters())
		}
	})

	// Create an answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	gatherComplete := make(chan string)
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		log.Println("ICE: ", candidate)
		if candidate == nil {
			gatherComplete <- "hi"
		}
	})
	//// Create channel that is blocked until ICE Gathering is complete
	//gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		return nil, err
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	return peerConnection.LocalDescription(), nil
}

// Create a 'server' peer connection, return the address descriptor.
// - uses pion/ice.Agent
// - DTLS cert fingerprint (SHA-256)
// - session descriptor:
//
// TODO:
// - session.URI
//
func InitWebRTCS(ug *ugatesvc.UGate, auth *auth.Auth) (*webrtc.PeerConnection, *webrtc.SessionDescription, error) {
	rtcg := &RTC{
		UGate: ug,
	}

	ug.Mux.HandleFunc("/wrtc/direct/", rtcg.RTCDirectHandler)

	/* Example:
	sessid, sessversion, sha,

	v=0
	o=- 7480214597711149065 1613143312 IN IP4 0.0.0.0
	s=-
	t=0 0
	a=fingerprint:sha-256 69:87:46:0F:EF:38:40:E1:84:92:CB:62:63:1E:95:4B:83:51:7C:03:D8:9F:13:FA:76:F8:EC:60:55:7C:6A:8F
	a=group:BUNDLE 0

	m=application 9 UDP/DTLS/SCTP webrtc-datachannel
	c=IN IP4 0.0.0.0
	a=setup:actpass
	a=mid:0
	a=sendrecv
	a=sctp-port:5000
	a=ice-ufrag:OifZfFtFkYbYlnpk
	a=ice-pwd:dUyGOKIYUcKuLoHQWhxbZNVYALrsJbAT
	*/
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
		BundlePolicy: webrtc.BundlePolicyMaxBundle,
	}
	// TODO: Certificates from the ugate EC256 key or core identity (ACME)
	//

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, nil,  err
	}

	ug.Mux.HandleFunc("/.dm/webrtc/local", func(w http.ResponseWriter, request *http.Request) {
		w.Write([]byte(peerConnection.LocalDescription().SDP))
	})

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})
	//peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
	//	log.Println(candidate)
	//	offer, _ := peerConnection.CreateOffer(nil)
	//	log.Println(offer)
	//})

	// Register data channel creation handling
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

		// Register channel opening handling
		d.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", d.Label(), d.ID())

			for range time.NewTicker(5 * time.Second).C {
				message := "hi"
				fmt.Printf("Sending '%s'\n", message)

				// Send the message as text
				sendErr := d.SendText(message)
				if sendErr != nil {
					panic(sendErr)
				}
			}
		})

		// Register text message handling
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))
		})
	})


	dc, err := peerConnection.CreateDataChannel("testdc", nil)
	dc.OnOpen(func() {
		dc.Send([]byte("Opened"))
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Println("Got ", msg)
	})


	offer, err := peerConnection.CreateOffer(nil)
	ug.Mux.HandleFunc("/.dm/webrtc/offer", func(w http.ResponseWriter, request *http.Request) {
		w.Write([]byte(offer.SDP))
	})

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}
	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)


	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	log.Println("Offer gather complete")
	return peerConnection, &offer, nil
}


