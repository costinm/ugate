package ugatesvc

import (
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/costinm/ugate"
	"golang.org/x/net/http2"
)

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
type H2R struct {

}

// Reverse Accept dials a connection to addr, and registers a H2 SERVER
// conn on it. The other end will register a H2 client, and create streams.
// The client cert will be used to associate incoming streams, based on config or direct mapping.
// TODO: break it in 2 for tests to know when accept is in effect.
func (t *H2Transport) maintainRemoteAccept(host, key string) error {
	if _, f := t.ug.Config.H2R[host]; !f {
		return errors.New("not configured")
	}

	//ctx := context.Background()
	//ctx, ctxCancel = context.WithTimeout(ctx, 5*time.Second)

	var addr string

	// addr is a hostname
	dm := t.ug.GetNode(host)
	if dm != nil {
		addr = dm.Addr
	} else {
		if key == "" {
			addr = net.JoinHostPort(host, "443")
		} else {
			addr = net.JoinHostPort(host, "15007")
		}
	}

	str, err := t.ug.DialTLS(addr, []string{"h2r", "h2"})
	if err != nil {
		// TODO: backoff
		log.Println("Failed to connect ", addr, err)
		time.AfterFunc(10000*time.Millisecond, func() {
			t.maintainRemoteAccept(host, key)
		})
		return err
	}

	p := str.TLS.NegotiatedProtocol
	if p == "h2r" {
		t.reverse[host] = str

		end := make(chan int)
		str.ReadCloser = func() {
			end <- 1
		}

		go func() {
			log.Println("H2R-Client: Reverse accept start ", str.RemoteAddr(), str.LocalAddr(), RemoteID(str))
			t.h2Server.ServeConn(
				str,
				&http2.ServeConnOpts{
					Handler: t, // Also plain text, needs to be upgraded
					Context: str.Context(),

					//Context: // can be used to cancel, pass meta.
					// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
				})
			log.Println("H2R-Client: Reverse accept closed")
			time.AfterFunc(500*time.Millisecond, func() {
				t.maintainRemoteAccept(host, key)
			})
		}()
		go func() {
			<-end
			log.Println("H2R-Client: Read Context done")
		}()

		// TODO: Wait for a 'hello' message
	} else if p == "h2" {

	}

	// TODO: H2, WS

	return nil
}

// TODO: implement H2 ClientConnPool

// Get a H2 client connection for connecting to a mesh host.
//
func (t *H2Transport) GetClientConn(req *http.Request, addr string) (*http2.ClientConn, error) {
	// The h2 Transport has support for dialing TLS, with the std handshake.
	// It is possible to replace Transport.DialTLS, used in clientConnPool
	// which tracks active connections. Or specify a custom conn pool.

	// addr is either based on req.Host or the resolved IP, in which case Host must be used for TLS verification.

	nid := t.ug.Auth.Host2ID(addr)
	dmn := t.ug.GetNode(nid)
	if dmn != nil {
		rt := dmn.H2r
		if rt != nil {
			if rtc, ok := rt.(*http2.ClientConn); ok {
				return rtc, nil
			}
		}
	}

	// TODO: reuse connection or use egress server
	// TODO: track it by addr
	tc, err := t.ug.DialTLS(addr, []string{"h2"})
	if err != nil {
		return nil, err
	}

	cc, err := t.ug.h2Handler.h2t.NewClientConn(tc)

	return cc, err
}

func (t *H2Transport) MarkDead(h2c *http2.ClientConn) {
	log.Println("Dead", h2c)
}

// HTTP round-trip using the mesh connections. Will use H2 and the mesh
// auth protocol, on the BTS port.
func (t *H2Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	cc, err := t.GetClientConn(req, req.Host)
	if err != nil {
		return nil, err
	}
	return cc.RoundTrip(req)
}

// WIP: can be registered using handlers-by-ALPN
// May not be needed if H2R works well.
//func (t *H2Transport) HandleSPDY(c MetaConn) error {
//	sc, err := h2raw.NewSPDY(c, true)
//	if err != nil {
//		return err
//	}
//	sc.serve()
//	return nil
//}

// Handle a raw reverse H2 connection, negotiated at TLS handshake level.
// The connection was accepted, but we act as client.
func (t *H2Transport) HandleH2R(str *ugate.Stream) error {
	// This is the H2 in reverse - start a TLS client conn, and keep  track of it
	// for forwarding to the dest.
	end := make(chan int)
	str.ReadCloser = func() {
		end <- 1
	}

	cc, err := t.h2t.NewClientConn(str)
	if err != nil {
		return err
	}

	k := RemoteID(str)
	n := t.ug.GetOrAddNode(k)
	n.H2r = cc
	t.H2R[k] = cc

	ra := str.RemoteAddr()
	// TODO: remember the IP, use it for the node ? Explicit registration may be better.

	log.Println("Accepted reverse conn ", ra, k)
	// Wait until t.MarkDead is called - or the con is closed
	<-end
	log.Println("H2R done", k)
	delete(t.H2R, k)
	n.H2r = nil
	return nil
}

