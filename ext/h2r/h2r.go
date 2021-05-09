package h2r

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"golang.org/x/net/http2"
)

// Original prototype - dropped due to lack of forward connection.
// Instead, h2r will be used to indicate the other end supports a modified
// H2 stack that simply allows server to originate requests, like Quic and
// WebRTC. For H2 we will us a POST to channel the reverse direction.
//
// H2R implements a transport using reverse HTTP/2, similar with 'ssh -R'.
// In this mode the 'client' A create a connection to the server 'B'.
// - If the ALPN negotiation result is 'h2r', B will initiate a HTTP/2 conn
// with A acting as server.
// - If ALPN negotiation is 'h2', A will initiate a HTTP/2 conn and issue
// a POST with /_h2r/id/[ID].  The handler for the POST will wrap a reverse
// HTTP/2 connection, with A acting as server.
// - Else a Websocket connection is attempted, to /_h2r/id/[ID]. A HTTP/2 conn
// from B to A over ws will be started.
//
// In all cases, B will forward requests for hostname ID to A.
//
// Authentication/Authorization are not included, a separate middleware or
// policy is required to verify A is authorized to reverse forward [ID].
//
// Original code:
// end := make(chan int)
//		str.ReadCloser = func() {
//			end <- 1
//		}
//
//		go func() {
//			log.Println("H2R-Client: Reverse accept start ", str.RemoteAddr(), str.LocalAddr(), RemoteID(str))
//			ug.H2Handler.h2Server.ServeConn(
//				str,
//				&http2.ServeConnOpts{
//					Handler: ug.H2Handler, // Also plain text, needs to be upgraded
//					Context: str.Context(),
//
//					//Context: // can be used to cancel, pass meta.
//					// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
//				})
//			log.Println("H2R-Client: Reverse accept closed")
//		}()
//		go func() {
//			<-end
//			log.Println("H2R-Client: Read Context done")
//		}()

type H2R struct {
	ug *ugatesvc.UGate

	// Transport object for http2 library
	h2t      *http2.Transport
	h2Server *http2.Server
	handler  http.Handler
}

func New(ug *ugatesvc.UGate) *H2R {
	h2t := &http2.Transport{
			ReadIdleTimeout: 10000 * time.Second,
			StrictMaxConcurrentStreams: false,
			AllowHTTP: true,
	}
	h2r := &H2R{ug: ug, handler: ug.H2Handler, h2t: h2t, h2Server: &http2.Server{}}
	ug.Mux.HandleFunc("/h2r/", h2r.HandleH2R)
	ug.MuxDialers["h2r"] = h2r
	return h2r
}

// H2RMux is a mux using H2 frames.
// Based on ALPN, may use raw frames.
type H2RMux struct {
	*http2.ClientConn

	tlsStr *ugate.Stream
	dm *ugate.DMNode

	// Raw frame support
	m       sync.RWMutex
	framer  *http2.Framer
	streams map[uint32]*H2Stream
	handleStream func(*H2Stream)
	nextStreamID uint32
}


// DialMUX creates one connection to a mesh node, using one of the
// supported multiplex protocols.
func (t *H2R) DialMux(ctx context.Context, dm *ugate.DMNode, meta http.Header, ev func(t string, stream *ugate.Stream)) (ugate.Muxer, error) {
	// TODO: try all published addresses, including all protos
	addr := dm.Addr

	str, err := t.ug.DialTLS(ctx, addr, []string{ "h2r",  "h2"})
	if err != nil {
		log.Println("Failed to connect ", addr, err)
		return nil, err
	}

	// Callback when the stream is closed, notify end.
	str.ReadCloser = func() {
		log.Println("H2R-Upstream closed")
	}


	proto := str.TLS.NegotiatedProtocol
	if proto == "h2r" {
		// Future extension, no need for the reverse.
	}
	//

	// Forward connection to the node.
	cc, err := t.h2t.NewClientConn(str)
	if err != nil {
		log.Println("Failed to initiate h2 conn")
		return nil, err
	}

	// TODO: use MASQUE to detect support ?

	// Initial message on the connection is to setup the reverse pipe.
	// This in turn will call this node, to validate the connection.
	r, w := io.Pipe() // pipe.New()
	postR, _ := http.NewRequest("POST",
		"https://" + addr + "/h2r/", r)
	tok := t.ug.Auth.VAPIDToken(addr)
	postR.Header.Add("authorization", tok)

	res, err := cc.RoundTrip(postR)
	if err != nil {
		return nil, err
	}

	str = ugate.NewStreamRequestOut(postR, w, res, nil)

	log.Println("H2R-Client: POST Reverse accept start ",
		str.RemoteAddr(), str.LocalAddr(), ugatesvc.RemoteID(str))

	go func() {
		t.h2Server.ServeConn(
			str,
			&http2.ServeConnOpts{
				Handler: t.handler, // Also plain text, needs to be upgraded
				Context: str.Context(),

				//Context: // can be used to cancel, pass meta.
				// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
			})
		log.Println("H2R-Client: Reverse accept closed")
		dm.Muxer = nil
		t.ug.OnMuxClose(dm)
	}()

	h2rm := &H2RMux{dm: dm, tlsStr: str, ClientConn: cc,
		// Used for raw streams, in both directions
		framer:       http2.NewFramer(str, str),
		streams:      map[uint32]*H2Stream{},
		nextStreamID: 3,
	}
	dm.Muxer = h2rm

	return h2rm, nil
}


// HandleH2R takes a POST "/h2r/" request and set the stream as a H2 client connection.
//
// It will start by sending a test "id" request, and associate the muxed connection to the
// node.
//
// Blocks until str.Close().
func (t *H2R) HandleH2R(w http.ResponseWriter, r *http.Request) {
	// This is the H2 in reverse - start a TLS client conn, and keep  track of it
	// for forwarding to the dest.
	str := ugate.NewStreamRequest(r, w, nil)
	str.TLS = r.TLS // TODO: also get the ID from JWT

	end := make(chan int)

	// Callback when the stream is closed, notify end.
	str.ReadCloser = func() {
		end <- 1
	}

	cc, err := t.h2t.NewClientConn(str)
	if err != nil {
		return
	}

	k := ugatesvc.RemoteID(str) // mesh ID based on client cert.

	n := t.ug.GetOrAddNode(k)

	n.Muxer = cc

	ra := str.RemoteAddr()
	// TODO: remember the IP, use it for the node ? Explicit registration may be better.

	// TODO: use new URL
	r0, _ := http.NewRequest("GET", "http://localhost/_dm/id/U/" + t.ug.Auth.ID, nil)
	tok := t.ug.Auth.VAPIDToken(n.ID)
	r0.Header.Add("authorization", tok)
	res0, err := cc.RoundTrip(r0)
	if err != nil {
		log.Println("Reverse accept id err ", err, ra, k)
		return
	}
	upData, _ := ioutil.ReadAll(res0.Body)
	res0.Body.Close()

	log.Println("H2R start on ", t.ug.Auth.ID, "for", n.ID, k, ra, " -> ", string(upData))

	//go func() {
	//	for {
	//		time.Sleep(20 * time.Second)
	//		r0, _ := http.NewRequest("GET", "http://localhost/_dm/id/H2RS", nil)
	//		_, err := cc.RoundTrip(r0)
	//		if err != nil {
	//			log.Println("Reverse accept id ", err, ra, k)
	//			return
	//		}
	//		//err := cc.Ping(context.Background())
	//		//if err != nil {
	//		//	log.Println("Ping err")
	//		//}
	//	}
	//
	//}()

	// Wait until t.MarkDead is called - or the con is closed
	<-end

	n.Muxer = nil
	return
}
