package ugate

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"golang.org/x/net/http2"
)

// HTTP2 based transport, using the standard library in a custom mode.

// It also implements http.Handler, and can be registered with a HTTP/2 or HTTP/1 server.
// For HTTP/1 it will use websocket, with standard TLS and SPDY for crypto or mux.
// For HTTP/2 it will the normal connection if mTLS was negotiated.
// Otherwise will do a TLS+SPDY handshake for the POST method.
type H2Transport struct {
	ug     *UGate
	Prefix string

	httpListener *listener
	h2t          *http2.Transport
	h2Server     *http2.Server

	ALPNHandlers map[string]ConHandler

	// Key is a Host:, as it would show in SOCKS and SNI
	H2R map[string]*http2.ClientConn

	reverse map[string]*Stream
	fs      http.Handler
}

func NewH2Transport(ug *UGate) (*H2Transport, error) {
	h2 := &H2Transport{
		ug:           ug,
		httpListener: newListener(),
		h2t: &http2.Transport{
			ReadIdleTimeout: 10 * time.Second,
		},
		reverse: map[string]*Stream{},
		ALPNHandlers: map[string]ConHandler{

		},
		H2R: map[string]*http2.ClientConn{},
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
func (t *H2Transport) UpdateReverseAccept() {
	for addr, key := range t.ug.Config.H2R {
		t.maintainRemoteAccept(addr, key)
	}
	for addr, str := range t.reverse {
		if _, f := t.ug.Config.H2R[addr]; !f {
			log.Println("Closing removed upstream ", addr)
			str.Close()
			delete(t.reverse, addr)
		}
	}
}

// Common entry point for H1, H2 - both plain and tls
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
		tok, pub, err := CheckVAPID(vapidH[0], time.Now())
		if err == nil {
			Pub = pub
			SAN = tok.Sub
		}
	}

	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		pk1 := r.TLS.PeerCertificates[0].PublicKey
		Pub = PublicKeyBytesRaw(pk1)
		// TODO: Istio-style, signed by a trusted CA. This is also for SSH-with-cert
		sans, _ := GetSAN(r.TLS.PeerCertificates[0])
		if len(sans) > 0 {
			SAN = sans[0]
		}
	}

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

	//if r.ProtoMajor > 1 {
		if len(parts) > 2 {
			if parts[1] == "h2r" {
				// Regular H2 request, the ALPN negotiation failed due to infrastructure.
				// Like WS, use one or more H2 forward streams to do the reverse H2.
				// WIP:
			} else if parts[1] == "dm" {
				r.Host = parts[2]
				r.URL.Scheme = "https"
				r.URL.Path = strings.Join(parts[3:], "/")
				str := NewStreamRequest(r, w, nil)
				str.postDial = func(conn net.Conn, err error) {
					if err != nil {
						w.Header().Add("Error", err.Error())
						w.WriteHeader(500)
						w.(http.Flusher).Flush()
						return
					}
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
				}

				l.ug.HandleVirtualIN(str)
				return
			} else if parts[1] == "tcp" {
				// Will use r.Host to find the destination - not sure if this is right.
				s := NewStreamRequest(r, w, nil)
				l.ug.HandleVirtualIN(s)
				return
			}
		}

	// HTTP/1.1
	if r.Method == "CONNECT" {
		// WS or HTTP Proxy
	}

	if strings.HasPrefix(r.RequestURI, "/ws") {

	} else {

	}

	// TODO: HTTP Proxy with absolute URL

	// Regular HTTP/1.1 methods
	l.ug.Mux.ServeHTTP(w,r)
}

// Handle accepted connection on a port declared as "http"
// Will sniff H2 and use the right handler.
//
// Ex: curl localhost:9080/debug/vars --http2-prior-knowledge
func (t *H2Transport) handleHTTPListener(pl *Listener, bconn *RawConn) error {
	err := pl.sniffH2(bconn)
	if err != nil {
		return err
	}
	ctx := bconn.Context()

	if bconn.Stream.Type == ProtoH2 {
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
		bconn.Stream.Type = ProtoHTTP
		// TODO: identify 'the proxy protocol'
		// port is marked as HTTP - assume it is HTTP
		t.httpListener.incoming <- bconn
		// TODO: wait for connection to be closed.
		<-ctx.Done()
	}

	return nil
}

// Implements the Handle connection interface for uGate.
//
func (t *H2Transport) Handle(c MetaConn) error {
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

	if str.Type == ProtoConnect {
		t.httpListener.incoming <- c
		// TODO: wait for connection to be closed.
		<-str.Context().Done()
	}

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
