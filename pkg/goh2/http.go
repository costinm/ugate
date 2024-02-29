package goh2

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/costinm/ssh-mesh/nio"
	"github.com/costinm/ugate"
	"golang.org/x/net/http2"
)

// HTTP2 based transport, using x/net/http2 library directly (instead of standard library).



// It also implements http.Handler, and can be registered with a HTTP/2 or HTTP/1 server.
// For HTTP/1 it will use websocket, with standard TLS and SPDY for crypto or mux.
// For HTTP/2 it will the normal connection if mTLS was negotiated.
// Otherwise will do a TLS+SPDY handshake for the POST method.
type H2Transport struct {
	ug *ugate.UGate

	// H2Transport object for http2 library
	h2t      *http2.Transport

	// Included file server, for UI.
	fs    http.Handler
	conns map[*http2.ClientConn]*ugate.MeshCluster

	m sync.RWMutex
}

func NewH2Transport(ug *ugate.UGate) (*H2Transport, error) {
	h2 := &H2Transport{
		ug:           ug,

		conns:        map[*http2.ClientConn]*ugate.MeshCluster{},
		h2t: &http2.Transport{
			ReadIdleTimeout:            10000 * time.Second,
			StrictMaxConcurrentStreams: false,
			AllowHTTP:                  true,
		},
	}

	if _, err := os.Stat("./www"); err == nil {
		h2.fs = http.FileServer(http.Dir("./www"))
		ug.Mux.Handle("/", h2.fs)
	}

	return h2, nil
}

// Using pipe: 345Mbps
//
// Not using:  440Mbps.
// The QUIC read buffer is 8k
const usePipe = false



// DialContext creates on TCP-over-H2 connection.
// rt is expected to be a H2 round tripper.
//
// If s is specified, it will be used as input.
func (t *H2Transport) DialContext(ctx context.Context, addr string, s io.Reader,
	rt http.RoundTripper) (nio.Stream, error){
	// We have an active reverse RoundTripper for the host.
	var in io.Reader
	var out io.WriteCloser

	h, port, _ := net.SplitHostPort(addr)

	if usePipe || s == nil {
		in, out = io.Pipe() // pipe.New()
		//in = p
		//out = p
	} else {
		in = s
	}

	// Regular TCP stream, upgraded to H2.
	// This is a simple tunnel, so use the right URL
	r1, err := http.NewRequestWithContext(ctx, "POST",
		"https://"+h+"/dm/127.0.0.1:"+port, in)

	// RoundTrip H2Transport guarantees this is set
	if r1.Header == nil {
		r1.Header = make(http.Header)
	}

	// RT client - forward the request.
	res, err := rt.RoundTrip(r1)
	if err != nil {
		log.Println("H2R error", addr, err)
		return nil, err
	}

	rs := nio.NewStreamRequest(r1, out, res)
	//if DebugClose {
	//	log.Println(rs.State().StreamId, "dialHbone.TUN: ", addr, r1.URL)
	//}
	return rs, nil
}

func (t *H2Transport) MarkDead(h2c *http2.ClientConn) {
	t.m.Lock()
	dmn := t.conns[h2c]
	if dmn != nil {
		dmn.RoundTripper = nil
		log.Println("Dead", dmn.ID, h2c)
	}
	t.m.Unlock()
}

// GetClientConn returns H2 multiplexed client connection for connecting to a mesh host.
//
// Part of x.net.http2.ClientConnPool interface.
// addr is a host:port, based on the URL host.
// The result implements RoundTrip interface.
func (t *H2Transport) GetClientConn(ctx context.Context, prot string, addr string) (*http2.ClientConn, error) {
	// The h2 H2Transport has support for dialing TLS, with the std handshake.
	// It is possible to replace H2Transport.DialContext, used in clientConnPool
	// which tracks active connections. Or specify a custom conn pool.

	// addr is either based on req.Host or the resolved IP, in which case Host must be used for TLS verification.
	host, _, _ := net.SplitHostPort(addr)

	nid := t.ug.Auth.Host2ID(addr)
	// TODO: if mesh node, don't attempt to dial directly
	dmn := t.ug.GetCluster(nid)
	if dmn != nil {
		rt := dmn.RoundTripper
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
	var tc nio.Stream
	var err error
	// TODO: use local announces
	// TODO: use VPN server for all or for mesh

	rc, err := t.ug.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	// Separate timeout for handshake and connection - ctx is used for the entire connection.
	to := t.ug.HandsahakeTimeout
	if to == 0 {
		to = 5 * time.Second
	}
	ctx1, cf := context.WithTimeout(ctx, to)
	defer cf()

	if prot == "http" {
		tc = nio.GetStream(rc, rc)
	} else {
		tlsc, err := t.ug.NewTLSConnOut(ctx1, rc, t.ug.Auth,
			host, []string{"h2"})
		if err != nil {
			return nil, err
		}
		tc = nio.NewStreamConn(tlsc)
	}

	// TODO: reuse connection or use egress server
	// TODO: track it by addr

	// This is using the native stack http2.H2Transport - implements RoundTripper
	cc, err := t.h2t.NewClientConn(tc)

	if dmn != nil {
		// Forward connection ok too.
		dmn.RoundTripper = cc
	}
	return cc, err
}

var NotFound = errors.New("not found")

// Ex: curl localhost:9080/debug/vars --http2-prior-knowledge

// Handle accepted connection on a port declared as "http"
//
//func (t *H2Transport) handleHTTPListener(bconn *InOutStream) error {
//
//	err := SniffH2(bconn)
//	if err != nil {
//		return err
//	}
//	ctx := bconn.Context()
//
//	if bconn.Type == ProtoH2 {
//		bconn.TLS = &tls.ConnectionState{
//			Version:     tls.VersionTLS12,
//			CipherSuite: tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
//		}
//		t.h2Server.ServeConn(
//			bconn,
//			&http2.ServeConnOpts{
//				Handler: t,   // Also plain text, needs to be upgraded
//				Context: ctx, // associated with the stream, with cancel
//
//				//Context: // can be used to cancel, pass meta.
//				// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
//			})
//	} else {
//		bconn.Type = ProtoHTTP
//		// TODO: identify 'the proxy protocol'
//		// port is marked as HTTP - assume it is HTTP
//		t.httpListener.incoming <- bconn
//		// TODO: wait for connection to be closed.
//		<-ctx.Done()
//	}
//
//	return nil
//}

//func SniffH2(s *InOutStream) error {
//	var proto string
//
//	for {
//		buf, err := s.Fill(0)
//		if err != nil {
//			return err
//		}
//
//		if ix := bytes.IndexByte(buf, '\n'); ix >= 0 {
//			if bytes.Contains(buf, []byte("HTTP/1.1")) {
//				proto = ProtoHTTP
//				break
//			}
//		}
//		if ix := bytes.IndexByte(buf, '\n'); ix >= 0 {
//			if bytes.Contains(buf, []byte("HTTP/2.0")) {
//				proto = ProtoH2
//				break
//			}
//		}
//	}
//
//	s.Type = proto
//
//	return nil
//}



// FindMux - find an EndpointCon that is able to accept new connections.
// Will also dial a connection as needed, and verify the mux can accept a new connection.
// LB should happen here.
// WIP: just one, no retry, only for testing.
// TODO: implement LB properly
//func (t *H2Transport) FindMux(ctx context.Context, c *ugate.MeshCluster) (*ugate.EndpointCon, error) {
//	if len(c.EndpointCon) == 0 {
//		var endp *ugate.Host
//		if len(c.Hosts) > 0 {
//			endp = c.Hosts[0]
//		} else {
//			endp = &ugate.Host{}
//			c.Hosts = append(c.Hosts, endp)
//		}
//
//		ep := &ugate.EndpointCon{
//			Cluster:  c,
//			Endpoint: endp,
//		}
//		c.EndpointCon = append(c.EndpointCon, ep)
//	}
//
//	ep := c.EndpointCon[0]
//
//	if ep.RoundTripper == nil {
//		h2c, err := t.GetClientConn(ctx, "http", c.Addr)
//		// TODO: on failure, try another endpoint
//		//err := t.dialH2ClientConn(ctx, ep)
//		if err != nil {
//			return nil, err
//		}
//		ep.RoundTripper = h2c
//	}
//	return ep, nil
//}

//func (l *H2Transport) Close() error {
//	return nil
//}
