package ugatesvc

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/pipe"
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

// Primary function for egress streams, after metadata has been parsed.
//
// Dial the target and proxy to it.
// - if Dest is a mesh node, use BTS
// - else use TCP proxy.
//
//
// str.Dest is the destination hostname:port or hostname.
//
func (ug *UGate) dialOut(str *ugate.Stream) error {
	// sets clientEventContextKey - if ctx is used for a round trip, will
	// set all data.
	// Will also make sure DNSStart, Connect, etc are set (if we want to)
	//ctx, cancel := context.WithCancel(bconn.Meta().Request.Context())
	//ctx := httptrace.WithClientTrace(bconn.Meta().Request.Context(),
	//	&bconn.Meta().ClientTrace)
	ctx := str.Context()
	//ctx = context.WithValue(ctx, "ugate.conn", bconn)
	var nc net.Conn
	var err error

	addr := str.Dest

	nid := ug.Auth.Host2ID(addr)

	// Destination is a mesh ID, not a host:port. Use the discovery or
	// existing reverse connections.
	// Dial out via an existing 'reverse h2' connection
	dmn := ug.GetNode(nid) // no port
	if dmn != nil {
		rt := dmn.H2r
		if rt != nil {
			// We have a H2C connection or can make RT.
			rw := str.HTTPResponse()
			var r1 *http.Request
			r := str.HTTPRequest()
			if rw != nil {
				// We have an active reverse H2 - use it
				r1, _ = CreateUpstreamRequest(nil, r)
			} else {
				// Regular TCP stream, upgraded to H2.
				r1, err = http.NewRequest(r.Method, r.URL.String(), r.Body)
				CopyRequestHeaders(r1.Header, r.Header)
			}
			r1.URL.Scheme = "https"
			// RT client - forward the request.
			res, err := rt.RoundTrip(r1)
			if err != nil {
				log.Println("H2R error", addr, err)
				str.WriteHeader(500)
				return err
			}
			CopyResponseHeaders(str.Header(), res.Header)
			str.PostDial(nc, err)
			str.WriteHeader(res.StatusCode)
			str.Flush()
			str.CopyBuffered(str, res.Body, true)
			log.Println("H2R: ", addr, r.URL, time.Since(str.Open))
			return nil
		}

		if dmn.Addr != "" {
			addr = dmn.Addr
		}

	}

	//err = NotFound
	//str.PostDial(nc, err)
	//return nil

	// TODO: use discovery to map VIPs or key-based hosts to real addr
	// TODO: use local announces
	// TODO: use VPN server for all or for mesh

	nc, err = ug.Dialer.DialContext(ctx, "tcp", addr)

	str.PostDial(nc, err)

	if err != nil {
		log.Println("Failed to connect ", str.Dest, err)
		return err
	}
	defer nc.Close()

	/////////////log.Println("Connected RA=", nc.RemoteAddr(), nc.LocalAddr())

	if dmn != nil {
		//var clientCon *RawConn
		//if clCon, ok := nc.(*RawConn); ok {
		//	clientCon = clCon
		//} else {
		//	clientCon = GetConn(nc)
		//	clientCon.Meta.Accepted = false
		//}
		lconn, err := ug.NewTLSConnOut(ctx, nc, ug.TLSConfig, "", nil)
		if err != nil {
			return err
		}
		nc = lconn
	}

	return str.ProxyTo(nc)
}

// OpenConn will make sure there is a secure connection to the node.
func (ug *UGate) CreateConn(ctx context.Context, dmn *ugate.DMNode) (error) {
	if dmn.H2r != nil {
		return nil
	}
		// TODO: if we don't have addr, use discovery

		// TODO: if discovery doesn't return an address, use upsteram gate.

		if dmn.Addr == "" {
			return NotFound
		}

		addr := dmn.Addr

		// TODO: use discovery to map VIPs or key-based hosts to real addr
		// TODO: use local announces
		// TODO: use VPN server for all or for mesh

		nc, err := ug.Dialer.DialContext(ctx, "tcp", addr)

		// TODO: caller must call: str.PostDial(nc, err)

		if err != nil {
			return  err
		}

		lconn, err := ug.NewTLSConnOut(ctx, nc, ug.TLSConfig, "",
			nil)
		if err != nil {
			nc.Close()
			return err
		}

		cc, err := ug.H2Handler.h2t.NewClientConn(lconn)
		if err != nil {
			nc.Close()
			return  err
		}
		dmn.H2r = cc

		return nil
}

// CreateStream will create a con to the node, if one doesn't exist, and open a multiplexed stream.
func (ug *UGate) CreateStream(ctx context.Context, dmn *ugate.DMNode, r1 *http.Request) (*ugate.Stream, error) {
	 err := ug.CreateConn(ctx, dmn)

	// We have an active reverse H2 - filter the headers and create a new request
	// r1, _ = CreateUpstreamRequest(nil, r1)

	if r1.Method == "" {
		r1.Method = "POST"
	}
	var out io.Writer
	out = nil
	if r1.Body == nil {
		if r1.Method != "GET" && r1.Method != "HEAD" {
			p := pipe.New()
			out = p
			r1.Body = p
		}
	}

	r1.URL.Scheme = "https"
	// RT client - forward the request.
	res, err := dmn.H2r.RoundTrip(r1)

	if err != nil {
		return nil, err
	}

	return ugate.NewStreamRequestOut(r1, out, res, nil), nil
}
