package ugatesvc

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime/debug"
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
	}
	h2.h2Server = &http2.Server{}

	if _, err := os.Stat("./www"); err == nil {
		h2.fs = http.FileServer(http.Dir("./www"))
		ug.Mux.Handle("/", h2.fs)
	}

	//ug.Mux.HandleFunc("/h2r/", h2.HandleH2R)

	ug.Mux.HandleFunc("/_dm/", ug.HandleID)
	ug.Mux.HandleFunc("/dm/", ug.HandleTCPProxy)

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

}

// Reverse Accept dials a connection to addr, and registers a H2 SERVER
// conn on it. The other end will register a H2 client, and create streams.
// The client cert will be used to associate incoming streams, based on config or direct mapping.
// TODO: break it in 2 for tests to know when accept is in effect.
func (t *H2Transport) maintainPinnedConnection(dm *ugate.DMNode, ev chan string) {
	// maintain while the host is in the 'pinned' list
	if _, f := t.ug.Config.H2R[dm.ID]; !f {
		return
	}

	//ctx := context.Background()
	if dm.Backoff == 0 {
		dm.Backoff = 1000 * time.Millisecond
	}

	ctx := context.TODO()
	//ctx, ctxCancel := context.WithTimeout(ctx, 5*time.Second)
	//defer ctxCancel()

	protos := t.ug.Config.TunProto
	if len(protos) == 0 {
		protos = []string{"quic", "h2r"}
	}
	var err error
	var muxer ugate.Muxer
	for _, k := range protos {
		muxer, err = t.ug.DialMUX(ctx, k, dm, nil)
		if err == nil {
			break;
		}
	}
	if err == nil {
		log.Println("UP: ", dm.Addr, muxer)
		// wait for mux to be closed
		dm.Backoff = 1000 * time.Millisecond
		return
	}

	log.Println("UP: err", dm.Addr, err, dm.Backoff)
	// Failed to connect
	if dm.Backoff < 15*time.Minute {
		dm.Backoff = 2 * dm.Backoff
	}

	time.AfterFunc(dm.Backoff, func() {
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
	var RemoteID string
	var SAN string
	defer func() {
		// TODO: add it to an event buffer
		l.ug.OnHClose("HTTP", RemoteID, SAN, r, time.Since(t0))
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

	// TODO: parse Envoy
	// https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/headers
	// In particular:
	//x-forwarded-client-cert: By=http://frontend.lyft.com;
	//  Hash=468..;
	//  URI=http://testclient.lyft.com,
	// By=http://backend.lyft.com;
	//  Hash=9ba61d6425303443;
	//  URI=http://frontend.lyft.com;DNS=foo.com

	vapidH := r.Header["Authorization"]
	if len(vapidH) > 0 {
		tok, pub, err := auth.CheckVAPID(vapidH[0], time.Now())
		if err == nil {
			RemoteID = auth.IDFromPublicKeyBytes(pub)
			SAN = tok.Sub
		}
	}

	tls := r.TLS
	// If the request was handled by normal uGate listener.
	us := r.Context().Value("ugate.stream")
	if ugs, ok := us.(*ugate.Stream); ok {
		tls = ugs.TLS
		r.TLS = tls
	}
	// other keys:
	// - http-server (*http.Server)
	// - local-addr - *net.TCPAddr
	//

	if tls != nil && len(tls.PeerCertificates) > 0 {
		pk1 := tls.PeerCertificates[0].PublicKey
		RemoteID = auth.IDFromPublicKey(pk1)
		// TODO: Istio-style, signed by a trusted CA. This is also for SSH-with-cert
		sans, _ := auth.GetSAN(tls.PeerCertificates[0])
		if len(sans) > 0 {
			SAN = sans[0]
		}
	}
	// Using the 'from' header internally
	if RemoteID != "" {
		r.Header.Set("from", RemoteID)
	} else {
		r.Header.Del("from")
	}

	if ugate.DebugClose {
		log.Println("HTTP-Start ", r.Method, r.URL, r.Proto, r.Header, RemoteID, SAN, r.RemoteAddr)
	}

	// TODO: authz for each case !!!!

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

	if r.Method == "CONNECT" {
		l.ug.HandleTCPProxy(w, r)
		return
	}

	l.ug.Mux.ServeHTTP(w,r)
}

// Handle accepted connection on a port declared as "http"
// Will sniff H2 and http/1.1 and use the right handler.
//
// Ex: curl localhost:9080/debug/vars --http2-prior-knowledge
func (t *H2Transport) handleHTTPListener(pl *ugate.Listener, acceptedCon net.Conn) error {
	bconn := ugate.GetStream(acceptedCon, acceptedCon)
	t.ug.OnStream(bconn)
	defer t.ug.OnStreamDone(bconn)

	err := SniffH2(bconn)
	if err != nil {
		return err
	}
	ctx := bconn.Context()

	if bconn.Type == ugate.ProtoH2 {
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
		bconn.Type = ugate.ProtoHTTP
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


func SniffH2(s *ugate.Stream) error {
	var proto string

	for {
		buf, err := s.Fill(0)
		if err != nil {
			return err
		}

		if ix := bytes.IndexByte(buf, '\n'); ix >=0 {
			if bytes.Contains(buf, []byte("HTTP/1.1")) {
				proto = ugate.ProtoHTTP
				break
			}
		}
		if ix := bytes.IndexByte(buf, '\n'); ix >=0 {
			if bytes.Contains(buf, []byte("HTTP/2.0")) {
				proto = ugate.ProtoH2
				break
			}
		}
	}

	s.Type = proto

	return nil
}


// Handle implements the connection interface for uGate, for HTTPS
// listeners.
//
// Blocking.
func (t *H2Transport) HandleHTTPS(c *ugate.Stream) error {
	// http2 and http expect a net.Listener, and do their own accept()
	str := c
	if str.TLS != nil && str.TLS.NegotiatedProtocol == "h2" {
		t.h2Server.ServeConn(
			c,
			&http2.ServeConnOpts{
				Handler: t,                  // Also plain text, needs to be upgraded
				Context: c.Context(), // associated with the stream, with cancel

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

type HttpClientStream struct {
	ugate.Stream
	*http.Response
	request *http.Request
}

func NewHttpClientStream(s *ugate.Stream) *HttpClientStream {
	h := &HttpClientStream{
	}
	return h
}

type HttpServerStream struct {
	ugate.Stream
	http.ResponseWriter
	request *http.Request
}

func NewHttpServerStream(s *ugate.Stream) *HttpServerStream {
	h := &HttpServerStream{
	}
	return h
}
