package ugatesvc

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/auth"
	"golang.org/x/net/http2"
)

// HTTP2 based transport, using the standard library.
// This handles the main https port (BTS), as well as QUIC/H3
// Will authenticate the request if possible ( JWT or mTLS ).

// It also implements http.Handler, and can be registered with a HTTP/2 or HTTP/1 server.
// For HTTP/1 it will use websocket, with standard TLS and SPDY for crypto or mux.
// For HTTP/2 it will the normal connection if mTLS was negotiated.
// Otherwise will do a TLS+SPDY handshake for the POST method.
type H2Transport struct {
	ug *UGate

	// the 'listener' object accepting
	httpListener *listener

	// Transport object for http2 library
	h2t      *http2.Transport
	h2Server *http2.Server

	// Client streams connected to 'upstream' servers.
	// Connections will be maintained, with exp. backoff.
	Reverse map[string]*ugate.Stream

	// Included file server, for UI.
	fs    http.Handler
	conns map[*http2.ClientConn]*ugate.DMNode

	m sync.RWMutex
}

func NewH2Transport(ug *UGate) (*H2Transport, error) {
	h2 := &H2Transport{
		ug:           ug,
		httpListener: newListener(),
		conns: map[*http2.ClientConn]*ugate.DMNode{},
		h2t: &http2.Transport{
			ReadIdleTimeout: 10000 * time.Second,
			StrictMaxConcurrentStreams: false,
			AllowHTTP: true,
		},
		Reverse: map[string]*ugate.Stream{},
	}
	h2.h2Server = &http2.Server{}

	if _, err := os.Stat("./www"); err == nil {
		h2.fs = http.FileServer(http.Dir("./www"))
		ug.Mux.Handle("/", h2.fs)
	}

	// Plain HTTP requests - we only care about CONNECT/ws
	go http.Serve(h2.httpListener, h2)

	return h2, nil
}

// UpdateReverseAccept updates the upstream accept connections, based on config.
// Should be called when the config changes
func (t *H2Transport) UpdateReverseAccept() {
	ev := make(chan string)
	for addr, key := range t.ug.Config.H2R {
		// addr is a hostname
		dm := t.ug.GetOrAddNode(addr)
		if dm.Addr == "" {
			if key == "" {
				dm.Addr = net.JoinHostPort(addr, "443")
			} else {
				dm.Addr = net.JoinHostPort(addr, "15007")
			}
		}

		go t.maintainPinnedConnection(dm, ev)
	}
	<- ev
	log.Println("maintainPinned connected for ", t.ug.Auth.VIP6)

	// We can also let them die
	for addr, str := range t.Reverse {
		if _, f := t.ug.Config.H2R[addr]; !f {
			log.Println("Closing removed upstream ", addr)
			str.Close()
			t.m.Lock()
			delete(t.Reverse, addr)
			t.m.Unlock()
		}
	}
}

// Common entry point for H1, H2 - both plain and tls
// Will do the 'common' operations - authn, authz, logging, metrics for all BTS and regular HTTP.
//
// Important:
// When using for BTS we need to work around golang http stack implementation.
// This should be used as fallback - QUIC and WebRTC have proper mux and TUN support.
// In particular, while H2 POST and CONNECT allow req/res Body to act as TCP stream,
// the closing (FIN/RST) are very tricky:
// - ResponseWriter (in BTS server) does not have a 'Close' method, it is closed after
//   the method returns. That means we can't signal the TCP FIN or RST, which breaks some
//   protocols.
// - The request must be fully consumed before the method returns.
//
func (l *H2Transport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	var Pub []byte
	var SAN string
	defer func() {
		// TODO: add it to an event buffer
		log.Println("HTTP", r, Pub, SAN,
			//h2c.SAN, h2c.ID(),
			r.RemoteAddr, r.URL, time.Since(t0))
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)

			debug.PrintStack()

			// find out exactly what the error was and set err
			var err error

			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}
			if err != nil {
				fmt.Println("ERRROR: ", err)
			}
		}
	}()

	vapidH := r.Header["Authorization"]
	if len(vapidH) > 0 {
		tok, pub, err := auth.CheckVAPID(vapidH[0], time.Now())
		if err == nil {
			Pub = pub
			SAN = tok.Sub
		}
	}

	tls := r.TLS
	us := r.Context().Value("ugate.stream")
	if ugs, ok := us.(*ugate.Stream); ok {
		tls = ugs.TLS
		r.TLS = tls
	}

	if tls != nil && len(tls.PeerCertificates) > 0 {
		pk1 := tls.PeerCertificates[0].PublicKey
		Pub = auth.PublicKeyBytesRaw(pk1)
		// TODO: Istio-style, signed by a trusted CA. This is also for SSH-with-cert
		sans, _ := auth.GetSAN(tls.PeerCertificates[0])
		if len(sans) > 0 {
			SAN = sans[0]
		}
	}

	log.Println("Received ", r.RequestURI)

	// TODO: authz for each case !!!!

	// For plain http requests, the context doesn't have useful data.
	parts := strings.Split(r.RequestURI, "/")

	// Explicit hostname - forwardTo the node.
	// The request may be a BTS TCP proxy or HTTP - either way forwarding is the same
	// if the dest is a mesh node (BTS).
	//
	// Special case: local plain text http or http2 server ( sidecar )
	host, found := l.ug.Config.Hosts[r.Host]
	if found {
		l.ForwardHTTP(w, r, host.Addr)
		return
	}

	// HTTP/1.1 ?
	if r.Method == "CONNECT" {
		// WS or HTTP Proxy
	}


	//if r.ProtoMajor > 1 {
		if len(parts) > 2 {
			if parts[1] == "_dm" {
				w.WriteHeader(201)
				w.Write([]byte(l.ug.Auth.ID))
				return
			} else 	if parts[1] == "h2r" {
				// Regular H2 request, the ALPN negotiation failed due to infrastructure.
				// Like WS, use one or more H2 forward streams to do the reverse H2.
				// WIP:
				str := ugate.NewStreamRequest(r, w, nil)
				str.TLS = r.TLS // TODO: also get the ID from JWT
				l.HandleH2R(str)
				log.Println("H2R closed ")
				return
			} else if parts[1] == "dm" {
				r1 := CreateUpstreamRequest(w, r)
				r1.Host = parts[2]
				r1.URL.Scheme = "https"
				r1.URL.Host = r1.Host
				r1.URL.Path = "/" + strings.Join(parts[3:], "/")

				str := ugate.NewStreamRequest(r1, w, nil)
				str.Dest = parts[2]
				str.PostDialHandler = func(conn net.Conn, err error) {
					if err != nil {
						w.Header().Add("Error", err.Error())
						w.WriteHeader(500)
						w.(http.Flusher).Flush()
						return
					}
					w.Header().Set("Trailer", "X-Close")
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
				}
				str.Dest = parts[2]

				l.ug.HandleVirtualIN(str)
				log.Println("TUN DONE ", parts)
				return
			} else if parts[1] == "tcp" {
				// Will use r.Host to find the destination - not sure if this is right.
				s := ugate.NewStreamRequest(r, w, nil)
				l.ug.HandleVirtualIN(s)
				return
			}
		}

	if strings.HasPrefix(r.RequestURI, "/ws") {

	} else {

	}

	// TODO: HTTP Proxy with absolute URL

	// Regular HTTP/1.1 methods
	l.ug.Mux.ServeHTTP(w,r)
}

// Handle accepted connection on a port declared as "http"
// Will sniff H2 and http/1.1 and use the right handler.
//
// Ex: curl localhost:9080/debug/vars --http2-prior-knowledge
func (t *H2Transport) handleHTTPListener(pl *ugate.Listener, bconn *ugate.BufferedStream) error {
	err := SniffH2(bconn)
	if err != nil {
		return err
	}
	ctx := bconn.Context()

	if bconn.Stream.Type == ugate.ProtoH2 {
		bconn.TLS = &tls.ConnectionState{
			Version: tls.VersionTLS12,
			CipherSuite: tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		}
		t.h2Server.ServeConn(
			bconn,
			&http2.ServeConnOpts{
				Handler: t,                  // Also plain text, needs to be upgraded
				Context: ctx, // associated with the stream, with cancel

				//Context: // can be used to cancel, pass meta.
				// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
			})
	} else {
		bconn.Stream.Type = ugate.ProtoHTTP
		// TODO: identify 'the proxy protocol'
		// port is marked as HTTP - assume it is HTTP
		t.httpListener.incoming <- bconn
		// TODO: wait for connection to be closed.
		<-ctx.Done()
	}

	return nil
}

var (
	// ClientPreface is the string that must be sent by new
	// connections from clients.
	h2ClientPreface = []byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")
)


func SniffH2(br *ugate.BufferedStream) error {
	br.Sniff()
	var proto string

	for {
		_, err := br.Fill()
		if err != nil {
			return err
		}
		off := br.End
		if ix := bytes.IndexByte(br.Buf[0:off], '\n'); ix >=0 {
			if bytes.Contains(br.Buf[0:off], []byte("HTTP/1.1")) {
				proto = ugate.ProtoHTTP
				break
			}
		}
		if off >= len(h2ClientPreface) {
			if bytes.Equal(br.Buf[0:len(h2ClientPreface)], h2ClientPreface) {
				proto = ugate.ProtoH2
				break
			}
		}
	}

	// All bytes in the buffer will be Read again
	br.Reset(0)
	br.Stream.Type = proto

	return nil
}


// Handle implements the connection interface for uGate, for HTTPS
// listeners.
//
// Blocking.
func (t *H2Transport) HandleHTTPS(c ugate.MetaConn) error {
	// http2 and http expect a net.Listener, and do their own accept()
	str := c.Meta()
	if str.TLS != nil && str.TLS.NegotiatedProtocol == "h2r" {
		return t.HandleH2R(str)
	}
	if str.TLS != nil && str.TLS.NegotiatedProtocol == "h2" {
		t.h2Server.ServeConn(
			c,
			&http2.ServeConnOpts{
				Handler: t,                  // Also plain text, needs to be upgraded
				Context: c.Meta().Context(), // associated with the stream, with cancel

				//Context: // can be used to cancel, pass meta.
				// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
			})
		return nil
	}

	// Else: HTTP/1.1
	t.httpListener.incoming <- c
	// TODO: wait for connection to be closed.
	<-str.Context().Done()

	return nil
}

func (l *H2Transport) Close() error {
	return nil
}


// Listener backed on an chan. Go HTTP stack requires a Listener
// implementation - use with http.Serve().
// Used with a port sniffer - i.e. a real listener will identify HTTP
// and H2 requests, and dispatch to the channel listener for HTTP.
type listener struct {
	l net.Listener

	closed   chan struct{}
	incoming chan net.Conn
}

func newListener() *listener {
	return &listener{
		incoming: make(chan net.Conn),
		closed:   make(chan struct{}),
	}
}

func (l *listener) Close() error {
	if l.l != nil {
		return l.l.Close()
	}
	l.closed <- struct{}{}
	return nil
}

func (l *listener) Addr() net.Addr {
	return l.l.Addr()
}

func (l *listener) Accept() (net.Conn, error) {
	for {
		select {
		case c, ok := <-l.incoming:

			if !ok {
				return nil, fmt.Errorf("listener is closed")
			}

			//if l.t.Gater != nil && !(l.t.Gater.InterceptAccept(c) && l.t.Gater.InterceptSecured(n.DirInbound, c.RemotePeer(), c)) {
			//	c.Close()
			//	continue
			//}
			return c, nil
		case <-l.closed:
			return nil, fmt.Errorf("listener is closed")
		}
	}
}
