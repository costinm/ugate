package ugatesvc

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/costinm/hbone/nio"
	"github.com/costinm/ugate"
	"golang.org/x/net/http2"
)

// After a Stream ( TCP+meta or HTTP ) is accepted/captured, we need to route it based on
// the config.
//
// Use cases:
// - Ingress: proxy to a local port on the same machine or in-process handler
// - Relay to mesh: use BTS or H2R to another gate. Dest is a mesh node.
// - Egress to internet - forward to a non-mesh node, on original port.
//   Can be TCP(incl TLS) or HTTP. Client MUST be local or trusted
// - Gateway to a non-mesh node - probably should not be supported, subcase
//  of 'ingress' but using a non-local address.

var NotFound = errors.New("not found")

func (ug *UGate) Dial(netw, addr string) (net.Conn, error) {
	return ug.DialContext(context.Background(), netw, addr)
}

// DialTLS opens a direct TLS connection using the dialer for TCP.
// No peer verification - the returned stream will have the certs.
// addr is a real internet address, not a mesh one.
//
// Used internally to create the raw TLS connections to both mesh
// and non-mesh nodes.
func (ug *UGate) DialTLS(ctx context.Context, addr string, alpn []string) (*nio.Stream, error) {
	ctx1, cf := context.WithTimeout(ctx, 5*time.Second)
	defer cf()
	tcpC, err := ug.parentDialer.DialContext(ctx1, "tcp", addr)
	if err != nil {
		return nil, err
	}
	// TODO: parse addr as URL or VIP6 extract peer WorkloadID
	t, err := ug.NewTLSConnOut(ctx1, tcpC, ug.TLSConfig, "", alpn)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// Primary function for egress streams, after metadata has been parsed.
//
// RoundTripStart the target and proxy to it.
// - if Dest is a mesh node, use BTS
// - else use TCP proxy.
//
//
// str.Dest is the destination hostname:port or hostname.
//

func (ug *UGate) DialAndProxy(str *nio.Stream) error {

	nc, err := ug.dial(context.Background(), str.Dest, str)
	str.PostDial(nc, err)
	if err != nil {
		// postDial will take care of sending error code.
		return err
	}
	defer nc.Close()

	// RoundTripStart may begin streaming from input connection to the dialed.
	// When dial return, the headers from dialed con are received.

	if ncs, ok := nc.(*nio.Stream); ok {
		if ncs.OutHeader != nil {
			CopyResponseHeaders(str.Header(), ncs.OutHeader)
		}
		str.Flush()
	}

	return str.ProxyTo(nc)
}

// Stream received via a BTS SNI route.
// Similar with dialAndProxy
func (ug *UGate) HandleSNIStream(str *nio.Stream) error {
	idx := strings.Index(str.Dest, ".")
	if idx > 0 {
		// Only SNI route to mesh nodes, ignore the domain name
		str.Dest = str.Dest[0:idx]
	}

	nc, err := ug.dial(context.Background(), str.Dest, str)
	str.PostDial(nc, err)

	if err != nil {
		// postDial will take care of sending error code.
		return err
	}
	defer nc.Close()

	return str.ProxyTo(nc)
}

// DialContext creates  connection to the remote addr, implements
// x.net.proxy.ContextDialer and ugate.ContextDialer.
//
// Used for integration with other libraries without dependencies.
// Does not support metatada or 0-RTT sending or piping an existing stream.
func (ug *UGate) DialContext(ctx context.Context, netw, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	n := ug.GetNode(host)
	if n != nil {
		// Mesh node
		return ug.dial(ctx, addr, nil)
	}

	//ug.dial(addr, )

	// Use the Dialer passed as an option, may do additional proxying
	// for the TCP connection.
	tcpC, err := ug.parentDialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	if "tls" == netw {
		// TODO: parse addr as URL or VIP6 extract peer WorkloadID
		return ug.NewTLSConnOut(ctx, tcpC, ug.TLSConfig, "", nil)
	}

	return tcpC, err
}

// DialMUX creates an association with the node, using one of the supported
// transports.
//
// The node should have at least the address or public key or hash populated.
//
// If it has real endpoint address - we can use the associated protocol.
// Else we can try all supported protos.
func (ug *UGate) DialMUX(ctx context.Context, net string, node *ugate.Cluster, ev func(t string, stream *nio.Stream)) (ugate.Muxer, error) {
	// TODO: list, try them all.
	rd := ug.MuxDialers[net]
	if rd == nil {
		return nil, errors.New("Not found " + net)
	}
	return rd.DialMux(ctx, node, nil, ev)
}

// This should be called when any MUX is created ( either way )
func (ug *UGate) OnMUX(node *ugate.Cluster) error {

	return nil
}

// Using pipe: 345Mbps
//
// Not using: constant 440Mbps.
// The QUIC read buffer is 8k
const usePipe = false

// Dial creates a stream to the given address.
func (ug *UGate) dial(ctx context.Context, addr string, s *nio.Stream) (net.Conn, error) {
	// sets clientEventContextKey - if ctx is used for a round trip, will
	// set all data.
	// Will also make sure DNSStart, Connect, etc are set (if we want to)
	//ctx, cancel := context.WithCancel(bconn.Meta().Request.Context())
	//ctx := httptrace.WithClientTrace(bconn.Meta().Request.Context(),
	//	&bconn.Meta().ClientTrace)
	//ctx = context.WithValue(ctx, "ugate.conn", bconn)
	var nc net.Conn
	var err error

	// Extract a mesh WorkloadID from the address, return the WorkloadID or ""
	nid := ug.Auth.Host2ID(addr)

	// Destination is a mesh WorkloadID, not a host:port. Use the discovery or
	// existing reverse connections.
	// RoundTripStart out via an existing 'reverse h2' connection
	dmn := ug.GetNode(nid) // no port
	if dmn != nil {
		rt := dmn.Muxer

		if rt != nil {
			// We have an active reverse RoundTripper for the host.
			var in io.Reader
			var out io.WriteCloser

			h, port, _ := net.SplitHostPort(addr)

			if dd, ok := rt.(ugate.StreamDialer); ok {
				return dd.DialStream(ctx, "127.0.0.1:"+port, s)
			}

			if usePipe || s == nil || s.In == nil {
				in, out = io.Pipe() // pipe.New()
				//in = p
				//out = p
			} else {
				in = s.In
			}

			// Regular TCP stream, upgraded to H2.
			// This is a simple tunnel, so use the right URL
			r1, err := http.NewRequestWithContext(ctx, "POST",
				"https://"+h+"/dm/127.0.0.1:"+port, in)

			// RoundTrip Transport guarantees this is set
			if r1.Header == nil {
				r1.Header = make(http.Header)
			}

			// RT client - forward the request.
			res, err := rt.RoundTrip(r1)
			if err != nil {
				log.Println("H2R error", addr, err)
				return nil, err
			}

			rs := nio.NewStreamRequestOut(r1, out, res, nil)
			if nio.DebugClose {
				log.Println(rs.StreamId, "dial.TUN: ", addr, r1.URL)
			}
			return rs, nil
		}

		if dmn.Addr != "" {
			addr = dmn.Addr
		}
	}

	// TODO: if it is a mesh node, create a connection !

	// TODO: use discovery to map VIPs or key-based hosts to real addr
	// TODO: use local announces
	// TODO: use VPN server for all or for mesh

	nc, err = ug.parentDialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		log.Println("Failed to connect ", addr, err)
		return nil, err
	}

	if dmn != nil {
		lconn, err := ug.NewTLSConnOut(ctx, nc, ug.TLSConfig, "", nil)
		if err != nil {
			return nil, err
		}
		nc = lconn
	}

	return nc, nil
}

// TODO: implement H2 ClientConnPool
// HTTP round-trip using the mesh connections. Will use H2 and the mesh
// auth protocol, on the BTS port.
func (ug *UGate) RoundTrip(req *http.Request) (*http.Response, error) {
	cc, err := ug.H2Handler.GetClientConn(req, req.Host)
	if err != nil {
		return nil, err
	}
	return cc.RoundTrip(req)
}

func (t *H2Transport) MarkDead(h2c *http2.ClientConn) {
	t.m.Lock()
	dmn := t.conns[h2c]
	if dmn != nil {
		dmn.Muxer = nil
		log.Println("Dead", dmn.ID, h2c)
	}
	t.m.Unlock()
}

// GetClientConn returns H2 multiplexed client connection for connecting to a mesh host.
//
// Part of x.net.http2.ClientConnPool interface.
// addr is a host:port, based on the URL host.
// The result implements RoundTrip interface.
func (t *H2Transport) GetClientConn(req *http.Request, addr string) (*http2.ClientConn, error) {
	// The h2 Transport has support for dialing TLS, with the std handshake.
	// It is possible to replace Transport.DialTLS, used in clientConnPool
	// which tracks active connections. Or specify a custom conn pool.

	// addr is either based on req.Host or the resolved IP, in which case Host must be used for TLS verification.

	nid := t.ug.Auth.Host2ID(addr)
	// TODO: if mesh node, don't attempt to dial directly
	dmn := t.ug.GetNode(nid)
	if dmn != nil {
		rt := dmn.Muxer
		if rt != nil {
			if rtc, ok := rt.(*http2.ClientConn); ok {
				return rtc, nil
			}
		}

		// TODO: if we don't have addr, use discovery

		// TODO: if discovery doesn't return an address, use upsteram gate.
		if dmn.Addr == "" {
			return nil, NotFound
		}
		// Real address -
		addr = dmn.Addr
	}
	var tc *nio.Stream
	var err error
	// TODO: use local announces
	// TODO: use VPN server for all or for mesh
	if req.URL.Scheme == "http" {
		rc, err := t.ug.DialContext(req.Context(), "tcp", addr)
		if err != nil {
			return nil, err
		}
		tc = nio.NewStream()
		tc.In = rc
		tc.Out = rc
	} else {
		tc, err = t.ug.DialTLS(req.Context(), addr, []string{"h2"})
		if err != nil {
			return nil, err
		}
	}

	// TODO: reuse connection or use egress server
	// TODO: track it by addr

	cc, err := t.ug.H2Handler.h2t.NewClientConn(tc)

	if dmn != nil {
		// Forward connection ok too.
		dmn.Muxer = cc
	}
	return cc, err
}

// CreateStream will open a stream to a node.
//
// The httpRequest may include metadata (headers), URL and a body.
// If a body is not provided, a pipe will be created - the result is a
// net.Stream and can be used as such.
//
// - http.Client is specialized on HTTP requests model - has a cookie jar,
// redirects, deals with idle connections, etc.
// - http.Client.Do() requires req.URL to be set, as well as Body
//   The request body is closed at the end of Do.
// - in http client, URL must be set and RequestURI must be empty.
// - RoundTripper takes a request and returns a *Response.
//
// http.Transport supports schem->RoundTripper map !
// It also supports HTTPS, HTTPProxies, connection caching - should be
// reused.

// http.Client also sets Authorization Basic, Cookies
// and has its own timeout.

// it also has a heuristic of looking for the type of the
// round tripper, checking for *http2.Transport name to
// guess if it supports context.

// TODO: implement RoundTrip, Muxer in UGate - as main interface
// TODO: convert client HttpRequest + Response to stream
// TODO: convert server HttpReq+Res to stream.
// TODO: 'messages' is a one-way http request
// TODO: support 'GET'
