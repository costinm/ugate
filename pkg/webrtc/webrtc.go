package webrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/pion/sctp"
	"github.com/pion/webrtc/v3"
)

// WIP: experiments with webRTC for data channels.
// WebRTC is bi-directional, like SSH - both ends can initiate connections.

type WebRTC struct {
	Mux    *http.ServeMux
	Name   string

}

type RTConn struct {
	// Peers
	Conn   map[string]sctp.Association
	pc1    *webrtc.PeerConnection
	offer  *webrtc.SessionDescription
	Name   string
	Client bool
}

// Returns a json with an offer for connecting this host, to allow a web or dmesh client
// to initiate a data channel.
// To reduce RT, the http client acts as WebRTC server, includes an offer.
func (rtc *WebRTC) RTCDirectHandler(w http.ResponseWriter, r *http.Request) {
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
	ans, err := rtc.AcceptPeering(&offer)

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


func (rtcg *WebRTC) Provision(i context.Context) error {
	if rtcg.Mux == nil {
		rtcg.Mux = http.NewServeMux()
	}
	//rtcg.Mux.HandleFunc("/.dm/webrtc/offer", func(w http.ResponseWriter, request *http.Request) {
	//	w.Write([]byte(rtcg.offer.SDP))
	//})
	//rtcg.Mux.HandleFunc("/.dm/webrtc/local", func(w http.ResponseWriter, request *http.Request) {
	//	w.Write([]byte(rtcg.pc1.LocalDescription().SDP))
	//})

	rtcg.Mux.HandleFunc("/wrtc/direct/", rtcg.RTCDirectHandler)

	return nil
}

// Create a 'server' peer connection, return the address descriptor.
// - uses pion/ice.Agent
// - DTLS cert fingerprint (SHA-256)
// - session descriptor:
//
// TODO:
// - session.URI
func (rtcg *RTConn) Start() error {

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
	// TODO: how do we handle multiple ( server style ?)
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return err
	}
	rtcg.pc1 = peerConnection
	slog0 := slog.With("name", rtcg.Name)

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

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		// "connecting stable"
		slog0.Info("OnICEConnectionStateChange", "connectionState", connectionState.String())
	})

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		slog0.Info("OnICECandidate", "candidate", candidate)
		//offer, _ := peerConnection.CreateOffer(nil)
		//log.Println(offer)
	})

	// Register data channel creation handling
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		slog1 := slog0.With("label", d.Label(), "id", d.ID())
		slog1.Info("OnDataChannel", )

		// Register channel opening handling
		d.OnOpen(func() {
			slog1.Info("OnOpen")
			go func() {
				for range time.NewTicker(5 * time.Second).C {
					message := "hi"
					// Send the message as text
					sendErr := d.SendText(message)
					if sendErr != nil {
						panic(sendErr)
					}
				}
			}()
		})

		// Register text message handling
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			slog1.Info("OnMessage", "data", string(msg.Data))
		})
	})


	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		slog0.Info("OnConnectionStateChange", "state", state, "sstate", peerConnection.SignalingState())

		if state == webrtc.PeerConnectionStateConnected {
			//log.Println(peerConnection.SCTP().H2Transport().GetRemoteCertificate())
			//lp, _ := peerConnection.SCTP().Transport().GetLocalParameters()
			//
			//slog0.Info("OnConnectionStateChange=Connected", "state", peerConnection.SCTP().Transport().GetRemoteCertificate(), "localp", lp )
		}
	})

	return nil
}

func (rtcg *RTConn) DataChannel(n string) error {
	dc, err := rtcg.pc1.CreateDataChannel(n, nil)
	dc.OnOpen(func() {
		log.Println("DC Open ")
		dc.Send([]byte("Opened"))
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Println("Got ", msg)
	})
	return err
}

func (rtcg *RTConn) Dial() error {
	rtcg.Start()
	dc, _ := rtcg.pc1.CreateDataChannel("dc0", nil)
	dc.OnOpen(func() {
		log.Println("DC Open ")
		dc.Send([]byte("Opened"))
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Println("Got ", msg)
	})


	offer, _ := rtcg.pc1.CreateOffer(nil)
	rtcg.offer = &offer

	peerConnection := rtcg.pc1
		// Sets the LocalDescription, and starts our UDP listeners
		if err := peerConnection.SetLocalDescription(offer); err != nil {
			return err
		}
		// Create channel that is blocked until ICE Gathering is complete
		gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

		// Block until ICE Gathering is complete, disabling trickle ICE
		// we do this because we only can exchange one signaling message
		// in a production application you should exchange ICE Candidates via OnICECandidate
		<-gatherComplete

	//log.Println("Offer gather complete", offer)
	// {offer v=0
	//o=- 4226443055588174689 1705330927 IN IP4 0.0.0.0
	//s=-
	//t=0 0
	//a=fingerprint:sha-256 2D:EF:8B:7A:82:09:0A:C4:7F:33:A1:55:E9:0B:CA:C4:FA:C6:5C:7B:BB:E6:AE:02:80:B1:3C:F3:A0:96:8E:7D
	//a=extmap-allow-mixed
	//a=group:BUNDLE 0
	//m=application 9 UDP/DTLS/SCTP webrtc-datachannel
	//c=IN IP4 0.0.0.0
	//a=setup:actpass
	//a=mid:0
	//a=sendrecv
	//a=sctp-port:5000
	//a=ice-ufrag:AyLwJUlXXeVvlzTD
	//a=ice-pwd:ggGigrcnoGALEkMDmBuWVNfAFwdicCnN
	// 0xc0000d6240}

	return nil
}

// Given an offer from a server (which needs to be started first, and send the offer via another transport ),
// dial the connection, returning a response that needs to be sent back the the 'server'.
func (wrtc *WebRTC) AcceptPeering(inOffer *webrtc.SessionDescription) (*webrtc.SessionDescription, error) {

	slog0 := slog.With("name", wrtc.Name)

	rtcg := &RTConn{Name: "accepted", Client: false}
	rtcg.Start()

	// Set the remote SessionDescription. Starts the transport already !
	err := rtcg.pc1.SetRemoteDescription(*inOffer)
	if err != nil {
		slog0.Error("SetRemoteDescription", "err", err)
		return nil, err
	}

	// After setRemoteDescription - we can use the remote IP as a candidate.
	// It would only work if remote has a public address - for example
	// IPv6.
	// Alternatively, the request could include a known 'gateway'
	//peerConnection.AddICECandidate(webrtc.ICECandidateInit{
	//	Candidate: "candidate:1932572598 2 udp 2130706431 2601:647:6100:449c:b62e:99ff:fead:e42 55174 typ host",
	//})


	// Create an answer
	answer, err := rtcg.pc1.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	gatherComplete := make(chan string)
	rtcg.pc1.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		log.Println("ICE: ", candidate)
		if candidate == nil {
			gatherComplete <- "hi"
		}
	})
	//// Create channel that is blocked until ICE Gathering is complete
	//gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	err = rtcg.pc1.SetLocalDescription(answer)
	if err != nil {
		return nil, err
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	return rtcg.pc1.LocalDescription(), nil
}
