package ugatesvc

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/pipe"
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

// Connect creates one connection to a mesh node, using one of the
// supported multiplex protocols.
func (ug *UGate) Connect(ctx context.Context, dm *ugate.DMNode, ev chan string) error {
	// TODO: try all published addresses, including all protos
	addr := dm.Addr
	t := ug.H2Handler

	str, err := ug.dialTLS(ctx, addr, []string{/*"h2r", */"h2"})
	if err != nil {
		log.Println("Failed to connect ", addr, err)
		return err
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
	cc, err := t.ug.H2Handler.h2t.NewClientConn(str)
	if err != nil {
		log.Println("Failed to initiate h2 conn")
		return err
	}

	//go func() {
	//	for {
	//		time.Sleep(50 * time.Second)
	//		r0, _ := http.NewRequest("GET", "http://localhost/_dm/id/UPSTREAM", nil)
	//		_, err := cc.RoundTrip(r0)
	//		if err != nil {
	//			log.Println("H2R upstream ", err)
	//			return
	//		}
	//		//err := cc.Ping(context.Background())
	//		//if err != nil {
	//		//	log.Println("Ping err")
	//		//}
	//	}
	//
	//}()

	t.m.Lock()
	t.Reverse[dm.ID] = str
	t.m.Unlock()

	// Set the mux, for future connections.
	dm.Muxer = cc

	// Initial message on the connection is to setup the reverse pipe.
	// This in turn will call this node, to validate the connection.
	p := pipe.New()
	postR, _ := http.NewRequest("POST", "https://localhost/h2r/", p)
	res, err := cc.RoundTrip(postR)
	if err != nil {
		return err
	}

	str = ugate.NewStreamRequestOut(postR, p, res, nil)

	log.Println("H2R-Client: POST Reverse accept start ",
		str.RemoteAddr(), str.LocalAddr(), RemoteID(str))

	if ev != nil {
		select {
			case ev <- "":
			default:
		}
	}

	t.h2Server.ServeConn(
		str,
		&http2.ServeConnOpts{
			Handler: t, // Also plain text, needs to be upgraded
			Context: str.Context(),

			//Context: // can be used to cancel, pass meta.
			// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
		})
	log.Println("H2R-Client: Reverse accept closed")
	dm.Muxer = nil
	return nil
}

// Reverse Accept dials a connection to addr, and registers a H2 SERVER
// conn on it. The other end will register a H2 client, and create streams.
// The client cert will be used to associate incoming streams, based on config or direct mapping.
// TODO: break it in 2 for tests to know when accept is in effect.
func (t *H2Transport) maintainPinnedConnection(dm *ugate.DMNode, ev chan string)  {
	// maintain while the host is in the 'pinned' list
	if _, f := t.ug.Config.H2R[dm.ID]; !f {
		return
	}

	ctx := context.Background()
	backoff := 3000 * time.Millisecond

	//ctx := context.Background()
	//ctx, ctxCancel = context.WithTimeout(ctx, 5*time.Second)

	// Blocking
	err := t.ug.Connect(ctx, dm, ev)
	if err != nil {
		if backoff < 15 * time.Minute {
			backoff = 2 * backoff
		}
	} else {
		backoff = 3000 * time.Millisecond
	}
	time.AfterFunc(backoff, func() {
		t.maintainPinnedConnection(dm, ev)
	})

	// p := str.TLS.NegotiatedProtocol
	//if p == "h2r" {
	//	// Old code used the 'raw' TLS connection to create a  server connection
	//	t.h2Server.ServeConn(
	//		str,
	//		&http2.ServeConnOpts{
	//			Handler: t, // Also plain text, needs to be upgraded
	//			Context: str.Context(),
	//
	//			//Context: // can be used to cancel, pass meta.
	//			// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
	//		})
	//}
}

// HandleH2R takes a stream ( handshaked over TLS or in a POST/CONNECT),
// and uses it to create a H2 RoundTripper, i.e. a client connection.
// Typically str is associated with the /h2r/ URL
//
// The connection was accepted/received, but we act as client.
// Blocks until str.Close().
func (t *H2Transport) HandleH2R(str *ugate.Stream) error {
	// This is the H2 in reverse - start a TLS client conn, and keep  track of it
	// for forwarding to the dest.
	end := make(chan int)

	// Callback when the stream is closed, notify end.
	str.ReadCloser = func() {
		end <- 1
	}

	cc, err := t.h2t.NewClientConn(str)
	if err != nil {
		return err
	}

	k := RemoteID(str) // mesh ID based on client cert.

	n := t.ug.GetOrAddNode(k)

	log.Println("Setting H2R on ", n.ID(), "for", t.ug.Auth.VIP6)
	n.Muxer = cc

	ra := str.RemoteAddr()
	// TODO: remember the IP, use it for the node ? Explicit registration may be better.

	// TODO: use new URL
	r0, _ := http.NewRequest("GET", "http://localhost/_dm/id/U/" + t.ug.Auth.ID, nil)
	res0, err := cc.RoundTrip(r0)
	if err != nil {
		log.Println("Reverse accept id ", err, ra, k)
		return err
	}
	upData, _ := ioutil.ReadAll(res0.Body)
	res0.Body.Close()
	log.Println("H2R-UP start ", k, ra, t.ug.Auth.ID, " -> ", string(upData))

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
	log.Println("H2R-UP end", k, ra, t.ug.Auth.ID)

	n.Muxer = nil
	return nil
}
