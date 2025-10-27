package h2r

import (
	"context"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/costinm/meshauth"
	nio "github.com/costinm/ssh-mesh/pkg/h2"

	"golang.org/x/net/http2"
)

// H2R provides a reverse HTTP2 server, accepting H2 connections on a remote
// gateway which terhminates TLS with a real certificate and applies policies,
// as usual - but instead of dialing a request it uses an accepted connection.
//
// In other words, it is the HTTP equivalent of `ssh -R` - using host header and
// L7 routing to send back H2C streams (HTTP/1.1 is converted to H2C).
//
// This is the server side, and only dealing with L7 - use SSH for L4 reverse
// listeners.
//
// The client connects to the H2R server, and starts listening using H2 protocol.
// The server maintains a list of connections, and forwards to the clients.
type H2R struct {

	m sync.RWMutex

	// H2Transport object for http2 library
	h2t      *http2.Transport
	h2Server *http2.Server

	peers map[string]*peer

	Handler  http.Handler
}

// peer is a remote HTTP server, using a -R connection over SSH or an
// H2/H2C connection, as a TCP client.
//
// It uses the standard library instead of x/net/http2 (which also works).
//
type peer struct {
	http.RoundTripper
}

func New() *H2R {
	h2t := &http2.Transport{
		ReadIdleTimeout:            10000 * time.Second,
		StrictMaxConcurrentStreams: false,
		AllowHTTP:                  true,
	}
	h2r := &H2R{h2t: h2t, h2Server: &http2.Server{}}

	return h2r
}

//func RemoteID(s nio.Stream) string {
//	tls := s.TLSConnectionState()
//	if tls != nil {
//		if len(tls.PeerCertificates) == 0 {
//			return ""
//		}
//		pk, err := certs.VerifyChain(tls.PeerCertificates)
//		if err != nil {
//			return ""
//		}
//
//		return certs.PublicKeyBase32SHA(pk)
//	}
//	return ""
//}


func (t *H2R) DialMux(ctx context.Context, dm *meshauth.Dest, meta http.Header, ev func(t string)) (http.RoundTripper, error) {
	// TODO: try all published addresses, including all protos
	addr := dm.Addr

	//str, err := t.ug.DialTLS(ctx, addr, []string{"h2r", "h2"})
	//if err != nil {
	//	log.Println("Failed to connect ", addr, err)
	//	return nil, err
	//}

	// Callback when the stream is closed, notify end.
	//str.ReadCloser = func() {
	//	log.Println("H2R-Upstream closed")
	//}

	//proto := str.TLS.NegotiatedProtocol
	//if proto == "h2r" {
	//	// Future extension, no need for the reverse.
	//}
	//

	// Forward connection to the node.
	//cc, err := t.h2t.NewClientConn(str)
	//if err != nil {
	//	log.Println("Failed to initiate h2 conn")
	//	return nil, err
	//}

	// TODO: use MASQUE to detect support ?

	// Initial message on the connection is to setup the reverse pipe.
	// This in turn will call this node, to validate the connection.
	r, w := io.Pipe() // pipe.New()
	postR, _ := http.NewRequestWithContext(ctx, "POST",
		"https://"+addr+"/h2r/", r)

	//tok := wp.VAPIDToken(t.ug.Mesh, addr)
	//postR.Header.Add("authorization", tok)

	res, err := dm.RoundTrip(postR)
	if err != nil {
		return nil, err
	}

	str := &nio.StreamHttpClient{
		Request: postR,
		Response: res,
		RequestInPipe: w,
	}

	log.Println("H2R-Client: POST Reverse accept start ")
	//, RemoteID(str))

	go func() {
		t.h2Server.ServeConn(
			str,
			&http2.ServeConnOpts{
				Handler: t.Handler, // Also plain text, needs to be upgraded
				Context: ctx,

				//Context: // can be used to cancel, pass meta.
				// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
			})
		log.Println("H2R-Client: Reverse accept closed")
		dm.RoundTripper = nil
		//t.ug.OnMuxClose(dm)
	}()

	//h2rm := &H2RMux{dm: dm, ClientConn: cc,
	//	// Used for raw streams, in both directions
	//	framer:       http2.NewFramer(str, str),
	//	streams:      map[uint32]*H2Stream{},
	//	nextStreamID: 3,
	//}
	//dm.Muxer = h2rm

	return nil, nil
}

func (t *H2R) RoundTripper(k string) http.RoundTripper {
	t.m.Lock()
	n, _ := t.peers[k]
	t.m.Unlock()
	return n
}

// HandleH2R takes a POST "/h2r/{client}/" request and set the stream as a H2 client connection.
//
// It will start by sending a test "id" request, and associate the muxed connection to the
// node.
//
// Blocks until str.Close().
func (t *H2R) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// This is the H2 in reverse - start a TLS client conn, and keep  track of it
	// for forwarding to the dest.
	str := nio.NewStreamServerRequest(r, w)

	end := make(chan int)

	// Callback when the stream is closed, notify end. Called by NewClientConn.
	str.ReadCloser = func() {
		end <- 1
	}

	// Create a 'client connection' - can be used to send requests to the peer.
	// Using /x/net/http2, this is as simple as using transport.NewClientConn.
	// This does a H2C handshake
	cc, err := t.h2t.NewClientConn(str)
	if err != nil {
		return
	}

	// Using native stack it's a bit more complicated:
	// - set a dialler that only returns this connection.
	// - use the native transport with H2C only.
	// - make a request to force a handshake and verify the peer identity.


	// Extract the peer identity
	k := r.PathValue("client")
	// RemoteID(str) // mesh WorkloadID based on client cert.

	// Find the cluster associated with the identity.
	t.m.Lock()
	n, _ := t.peers[k]
	if n == nil {
		n = &peer{}
		t.peers[k] = n
	}
	// Set the roundtripper on the cluster.

	n.RoundTripper = cc
	t.m.Unlock()

	ra := str.RemoteAddr()
	// TODO: remember the IP, use it for the node ? Explicit registration may be better.

	// TODO: use new URL
	//r0, _ := http.NewRequest("GET", "http://localhost/_dm/id/U/"+t.ug.Auth.ID, nil)
	//tok := t.ug.Auth.VAPIDToken(n.ID)
	//r0.Header.Add("authorization", tok)
	//res0, err := cc.RoundTrip(r0)
	//if err != nil {
	//	log.Println("Reverse accept id err ", err, ra, k)
	//	return
	//}
	//upData, _ := ioutil.ReadAll(res0.Body)
	//res0.Body.Close()

	log.Println(str.State().StreamId, "H2R start on ",  k, ra)

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

	//t.ug.RegisterEndpoint(n.WorkloadID)

	// Wait until t.MarkDead is called - or the con is closed
	<-end

	// TODO:
	//t.ug.UnRegisterEndpoint(n.WorkloadID)

	t.m.Lock()
	if n.RoundTripper == cc {
		n.RoundTripper = nil
	}
	t.m.Unlock()
	return
}

/*
2023-07: The 4th design:

- main reverse based on Quic and WebRTC, which have native support (bi-directional)
- on client side - explicit config for the upstream clusters/endpoints via label or dedicated port
	- client and maintain TLS or TCP connections with the peer IP
	- start a HTTPS listener, identical to HBONE port.
- on server side - dedicated port/service, TCP or Websocket
	- on accept, start a HTTP client connection.
    - keep a map of peerID to connections
    - when making calls to the peer, use the map - that's the main core integration.
- no more TLS hacks/handshakes
- swap roles instead of PUSH streams, no mixing of reverse and forward.
- maybe support the reverse roles over a H2 or websocket stream.

*/

// Original comments, using SNI routing and a separate reverse-role connection. Client->Server on different connection
// from Server->Client streams.
// H2R implements a reverse H2 connection:
// - Bob connects to the Gate's H2R port (or H2 port with ALPN=h2r), using mTLS,
// - gate authenticates bob, authorizes. Using the SNI in the TLS and cert.
// - Gate opens a client H2 connection on the accepted mTLS
// - Bob opens a H2 server connection on the dialed mTLS
// - Optional: Gate sends an 'init message', Bob responds. This may include additional meta.
//
// Either SNI or bob's cert SAN can be used to identify the cluster this belongs to.
//

// Alternative protocol using infra/POST (no changes to H2 or TLS, single connection, double mux):
// - Bob connects to regular hbone/POST port, starts a regular UGate H2 connection
// - Bob opens a stream (RoundTripStart)
// - Option 1: Gate opens an H2 client on the stream, uses stream headers to identify cluster
// - Option 2: Gate adds the stream to a 'listeners' list, forwards incoming stream directly
//   ( no encapsulation ).

// Even older docs:
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
// a POST with /_h2r/id/[WorkloadID].  The handler for the POST will wrap a reverse
// HTTP/2 connection, with A acting as server.
// - Else a Websocket connection is attempted, to /_h2r/id/[WorkloadID]. A HTTP/2 conn
// from B to A over ws will be started.
//
// In all cases, B will forward requests for hostname WorkloadID to A.
//
// Authentication/Authorization are not included, a separate middleware or
// policy is required to verify A is authorized to reverse forward [WorkloadID].
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

// WIP: RemoteForward is similar with ssh -R remotePort.
// Will use the H2R protocol to open a remote H2C connection
// attached to the Hbone remote server.
//func RemoteForward(hb *ugate.UGate, hg, sn, ns string) *ugate.EndpointCon {
//	attachC := &ugate.MeshCluster{
//		ID:   "h2r-" + hg,
//		Dest: meshauth.Dest{Addr: sn + "." + ns + ":15009", SNI: fmt.Sprintf("outbound_.%s_._.%s.%s.svc.cluster.local", "15009", sn, ns)},
//	}
//	hb.addCluster(attachC, &ugate.Host{
//		Address: attachC.Addr,
//		Labels: map[string]string{
//			"h2r": "1",
//		},
//	})
//
//	attachE := &ugate.EndpointCon{
//		Host: &ugate.Host{
//			Address: attachC.Addr,
//		},
//		Cluster: attachC,
//	}
//
//	//go func() {
//	//	_, err := DialH2R(context.Background(), attachE, hg)
//	//	log.Println("H2R connected", hg, err)
//	//}()
//
//	return attachE
//}

//// GetClientConn is called by http2.H2Transport, if H2Transport.RoundTrip is called (
//// for example used in a http.Client ). We are using the http2.ClientConn directly,
//// but this method may be needed if this library is used as a http client.
//func (hb *UGate) GetClientConn(req *http.Request, addr string) (*http2.ClientConn, error) {
//	c, err := hb.MeshCluster(req.Context(), addr)
//	if err != nil {
//		return nil, err
//	}
//
//	m, err := c.findMux(req.Context())
//	if err != nil {
//		return nil, err
//	}
//
//	return m.rt.(*http2.ClientConn), nil
//}
//
//func (hb *UGate) MarkDead(conn *http2.ClientConn) {
//	hb.m.Lock()
//	sni := hb.H2RConn[conn]
//
//	if sni != nil {
//		log.Println("H2RSNI: MarkDead ", sni)
//	}
//	delete(hb.H2RConn, conn)
//	hb.m.Unlock()
//
//}

//// HandleH2RConn takes a connection on the H2R port or on a stream and
//// implements a reverse connection.
//func (hb *UGate) HandlerH2RConn(conn net.Conn) {
//	conf := hb.Auth.GenerateTLSConfigServer()
//
//	tls := tls.Server(conn, conf)
//
//	err := nio.HandshakeTimeout(tls, hb.HandsahakeTimeout, conn)
//	if err != nil {
//		conn.Close()
//		return
//	}
//
//	// At this point we have the client identity, and we know it's in the trust domain and right CA.
//	// TODO: save the endpoint.
//
//	// TODO: construct the SNI header, save it in the map
//	// TODO: validate the trust domain, root cert, etc
//
//	sni := tls.ConnectionState().ServerName
//	if Debug {
//		log.Println("H2RSNI: accepted ", sni)
//	}
//	ctx := context.Background()
//
//	c, _ := hb.MeshCluster(ctx, sni)
//
//	// not blocking. Will write the 'preface' and start reading.
//	// When done, MarkDead on the conn pool in the transport is called.
//	rt, err := hb.h2t.NewClientConn(tls)
//	if err != nil {
//		conn.Close()
//		return
//	}
//
//	ec := &EndpointCon{
//		c:        c,
//		Host: &Host{Address: sni},
//		rt:       rt,
//	}
//	hb.m.Lock()
//	c.EndpointCon = append(c.EndpointCon, ec)
//	hb.m.Unlock()
//
//	hb.H2RConn[rt] = ec
//
//	// TODO: track the active connections in hb, for close purpose.
//}

// Reverse tunnel: create a persistent connection to a gateway, and
// accept connections over that connection.
//
// The gateway registers the current endpoint with it's own IP:port
// (for example WorkloadEntry or Host or in-memory ), and forwards accepted requests over the
// established connection.

// UpdateReverseAccept updates the upstream accept connections, based on config.
// Should be called when the config changes
//func (t *H2Transport) UpdateReverseAccept() {
//ev := make(chan string)
//for addr, key := range t.ug.Config.H2R {
//	// addr is a hostname
//	dm := t.ug.GetOrAddNode(addr)
//	if dm.Addr == "" {
//		if key == "" {
//			dm.Addr = net.JoinHostPort(addr, "443")
//		} else {
//			dm.Addr = net.JoinHostPort(addr, "15007")
//		}
//	}
//
//	go t.maintainPinnedConnection(dm, ev)
//}
//<-ev
//log.Println("maintainPinned connected for ", t.ug.Auth.VIP6)

//}

// Reverse Accept dials a connection to addr, and registers a H2 SERVER
// conn on it. The other end will register a H2 client, and create streams.
// The client cert will be used to associate incoming streams, based on config or direct mapping.
// TODO: break it in 2 for tests to know when accept is in effect.
//func KeepConnected(ug *ugate.UGate, dm *ugate.MeshCluster, ev chan string) {
//	// maintain while the host is in the 'pinned' list
//	//if _, f := t.ug.Config.H2R[dm.muxID]; !f {
//	//	return
//	//}
//
//	//ctx := context.Background()
//	if dm.Backoff == 0 {
//		dm.Backoff = 1000 * time.Millisecond
//	}
//
//	ctx := context.TODO()
//	//ctx, ctxCancel := context.WithTimeout(ctx, 5*time.Second)
//	//defer ctxCancel()
//
//	var err error
//	var muxer http.RoundTripper
//	muxer, err = ug.DialMUX(ctx, "", dm, nil)
//	if err == nil {
//		log.Println("UP: ", dm.Addr, muxer)
//		// wait for mux to be closed
//		dm.Backoff = 1000 * time.Millisecond
//		return
//	}
//
//	log.Println("UP: err", dm.Addr, err, dm.Backoff)
//	// Failed to connect
//	if dm.Backoff < 15*time.Minute {
//		dm.Backoff = 2 * dm.Backoff
//	}
//
//	time.AfterFunc(dm.Backoff, func() {
//		KeepConnected(ug, dm, ev)
//	})

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
//}
