package ugate

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

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
	Mux    *http.ServeMux

	httpListener *listener
	h2t          *http2.Transport
	h2Server     *http2.Server

	ALPNHandlers map[string]ConHandler
	H2R          map[string]*http2.ClientConn
}

func NewH2Transport(ug *UGate) (*H2Transport, error) {
	h2 := &H2Transport{
		ug: ug,
		httpListener: newListener(),
		h2t: &http2.Transport{},
		Mux: http.NewServeMux(),
		ALPNHandlers: map[string]ConHandler{

		},
		H2R: map[string]*http2.ClientConn{},
	}
	h2.h2Server = &http2.Server{}


	// Plain HTTP requests - we only care about CONNECT/ws
	go http.Serve(h2.httpListener, h2)

	return h2, nil
}

// Reverse Accept dials a connection to addr, and registers a H2 SERVER
// conn on it. The other end will register a H2 client, and create streams.
// The client cert will be used to associate incoming streams, based on config or direct mapping.
// TODO: break it in 2 for tests to know when accept is in effect.
func (t *H2Transport) ReverseAccept(addr string) error {
	ctx := context.Background()
	tc, err := t.ug.DialContext(ctx, "h2r", addr)
	if err != nil {
		return err
	}
	t.h2Server.ServeConn(
		tc,
		&http2.ServeConnOpts{
			Handler: t,  // Also plain text, needs to be upgraded
			Context: ctx,

			//Context: // can be used to cancel, pass meta.
			// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
		})
	log.Println("Reverse accept closed")
	// TODO: keep alive
	return nil
}

// TODO: implement H2 ClientConnPool

// Get a client connection for connecting to a host.
func (t *H2Transport) GetClientConn(req *http.Request, addr string) (*http2.ClientConn, error) {
	// The h2 Transport has support for dialing TLS, with the std handshake.
	// It is possible to replace Transport.DialTLS, used in clientConnPool
	// which tracks active connections. Or specify a custom conn pool.

	// TODO: reuse connection or use egress server
	// TODO: track it by addr
	tc, err := t.ug.DialContext(req.Context(), "tls", addr)
	if err != nil {
		return nil, err
	}

	cc, err :=  t.ug.h2Handler.h2t.NewClientConn(tc)

	return cc, err
}

func (t *H2Transport) MarkDead(h2c *http2.ClientConn) {
	log.Println("Dead", h2c)
}

// HTTP round-trip using the mesh connections. Will use mTLS and internal
// routing to create the TLS stream.
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

// Handle a reverse H2 connection, negotiated at TLS handshake level.
// The connection was accepted, but we act as client. GetClientConn will use the reverse conn.
func (t *H2Transport) HandleH2R(c MetaConn) error {
	// This is the H2 in reverse - start a TLS client conn, and keep  track of it
	// for forwarding to the dest.
	cc, err := t.h2t.NewClientConn(c)
	if err != nil {
		return err
	}
	k := c.Meta().RemoteID()
	t.H2R[k] = cc
	// Wait until t.MarkDead is called - or the con is closed
	rc := c.Meta().Request.Context()
	<- rc.Done()
	log.Println("H2R done", k)
	delete(t.H2R, k)
	return nil
}

// Implements the Handle connection interface for uGate.
//
func (t *H2Transport) Handle(c MetaConn) error {
	// http2 and http expect a net.Listener, and do their own accept()
	meta := c.Meta()
	if meta.Request.TLS != nil && meta.Request.TLS.NegotiatedProtocol == "h2r" {
		return t.HandleH2R(c)
	}
	if meta.Request.TLS != nil && meta.Request.TLS.NegotiatedProtocol == "h2" {
		t.h2Server.ServeConn(
			c,
			&http2.ServeConnOpts{
				Handler: t,  // Also plain text, needs to be upgraded
				Context: c.Meta().Request.Context(),

				//Context: // can be used to cancel, pass meta.
				// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
			})
		return nil
	}

	if c.Meta().Type == ProtoConnect {
		t.httpListener.incoming <- c
		// TODO: wait for connection to be closed.
		<- c.Meta().Request.Context().Done()
	}

	return nil
}

func (l *H2Transport) Close() error {
	return nil
}

// Common entry point for H1, H2, both plain and tls
func (l *H2Transport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Request ", r)

	// For plain http requests, the context doesn't have useful data.

	if r.ProtoMajor > 1 {
		parts := strings.Split(r.RequestURI, "/")
		if len(parts) > 2 {
			if parts[1] == "h2r" {
				// Regular H2 request, the ALPN negotiation failed due to infrastructure.
				// Like WS, use one or more H2 forward streams to do the reverse H2.
				// WIP:
			} else if parts[1] == "dm" {
				r.Host = parts[2]
				r.URL.Scheme = "https"
				r.URL.Path = strings.Join(parts[3:], "/")
				str := l.ug.NewStreamRequest(r, w, nil)
				str.postDial = func(conn net.Conn, err error) {
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
				}

				l.ug.HandleVirtualIN(str)
				return
			} else if parts[1] == "tcp" {
				// Will use r.Host to find the destination - not sure if this is right.
				s := l.ug.NewStreamRequest(r, w, nil)
				l.ug.HandleVirtualIN(s)
				return
			}
		}
		l.Mux.ServeHTTP(w, r)
		return
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

}

// createUpstremRequest shallow-copies r into a new request
// that can be sent upstream.
//
// Derived from reverseproxy.go in the standard Go httputil package.
// Derived from caddy
func createUpstreamRequest(rw http.ResponseWriter, r *http.Request) (*http.Request, context.CancelFunc) {
	// Original incoming DmDns request may be canceled by the
	// user or by std lib(e.g. too many idle connections).
	ctx, cancel := context.WithCancel(r.Context())
	if cn, ok := rw.(http.CloseNotifier); ok {
		notifyChan := cn.CloseNotify()
		go func() {
			select {
			case <-notifyChan:
				cancel()
			case <-ctx.Done():
			}
		}()
	}

	outreq := r.WithContext(ctx) // includes shallow copies of maps, but okay

	// We should set body to nil explicitly if request body is empty.
	// For DmDns requests the Request Body is always non-nil.
	if r.ContentLength == 0 {
		outreq.Body = nil
	}

	// We are modifying the same underlying map from req (shallow
	// copied above) so we only copy it if necessary.
	copiedHeaders := false

	// Remove hop-by-hop headers listed in the "Connection" header.
	// See RFC 2616, section 14.10.
	if c := outreq.Header.Get("Connection"); c != "" {
		for _, f := range strings.Split(c, ",") {
			if f = strings.TrimSpace(f); f != "" {
				if !copiedHeaders {
					outreq.Header = make(http.Header)
					copyHeader(outreq.Header, r.Header)
					copiedHeaders = true
				}
				outreq.Header.Del(f)
			}
		}
	}

	// Remove hop-by-hop headers to the backend. Especially
	// important is "Connection" because we want a persistent
	// connection, regardless of what the client sent to us.
	for _, h := range hopHeaders {
		if outreq.Header.Get(h) != "" {
			if !copiedHeaders {
				outreq.Header = make(http.Header)
				copyHeader(outreq.Header, r.Header)
				copiedHeaders = true
			}
			outreq.Header.Del(h)
		}
	}

	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		// If we aren't the first proxy, retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := outreq.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		outreq.Header.Set("X-Forwarded-For", clientIP)
	}

	return outreq, cancel
}

// Hop-by-hop headers. These are removed when sent to the backend in createUpstreamRequest
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Alt-Svc",
	"Alternate-Protocol",
	"Connection",
	"Keep-Alive",
	"HTTPGate-Authenticate",
	"HTTPGate-Authorization",
	"HTTPGate-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
	"Te",                  // canonicalized version of "TE"
	"Trailer",             // not Trailers per URL above; http://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",
}

// used in createUpstreamRequetst to copy the headers to the new req.
func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		if _, ok := dst[k]; ok {
			// skip some predefined headers
			// see https://github.com/mholt/caddy/issues/1086
			if _, shouldSkip := skipHeaders[k]; shouldSkip {
				continue
			}
			// otherwise, overwrite to avoid duplicated fields that can be
			// problematic (see issue #1086) -- however, allow duplicate
			// Server fields so we can see the reality of the proxying.
			if k != "Server" {
				dst.Del(k)
			}
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// skip these headers if they already exist.
// see https://github.com/mholt/caddy/pull/1112#discussion_r80092582
var skipHeaders = map[string]struct{}{
	"Content-Type":        {},
	"Content-Disposition": {},
	"accept-Ranges":       {},
	"Set-Cookie":          {},
	"Cache-Control":       {},
	"Expires":             {},
}

// Used by both ForwardHTTP and ForwardMesh, after RoundTrip is done.
// Will copy response headers and body
func SendBackResponse(w http.ResponseWriter, r *http.Request,
		res *http.Response, err error) {

	if err != nil {
		if res != nil {
			CopyHeaders(w.Header(), res.Header)
			w.WriteHeader(res.StatusCode)
			io.Copy(w, res.Body)
			log.Println("Got ", err, res.Header)
		} else {
			http.Error(w, err.Error(), 500)
		}
		return
	}

	origBody := res.Body
	defer origBody.Close()

	CopyHeaders(w.Header(), res.Header)
	w.WriteHeader(res.StatusCode)

	stats := &Stream{}
	n, err := stats.CopyBuffered(w, res.Body, true)

	log.Println("Done: ", r.URL, res.StatusCode, n, err)
}


// Also used in httpproxy_capture, for forward http proxy
func CopyHeaders(dst, src http.Header) {
	for k, _ := range dst {
		dst.Del(k)
	}
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}


// Listener backed on an chan.
type listener struct {
	l net.Listener

	closed   chan struct{}
	incoming chan net.Conn
}

func newListener() *listener {
	return &listener {
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
