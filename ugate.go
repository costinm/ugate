package ugate

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/costinm/meshauth"
	sshd "github.com/costinm/ssh-mesh"
	"github.com/costinm/ssh-mesh/nio"
	"github.com/costinm/ugate/pkg/httpwrapper"
	msgs "github.com/costinm/ugate/webpush"

	"github.com/costinm/ssh-mesh/nio/syscall"
)

// Debug for dev support, will log verbose info.
// Avoiding dependency on logging - eventually a trace interface will be provided
// so any logger can be used.
var Debug = false

// MeshSettings holds the settings for a mesh node.
type MeshSettings struct {
	// SSHConfig includes MeshCfg - which defines the authentication.
	//
	// Current ugate mesh 'core' protocol is SSH, other protocols are bridged/gateway-ed
	// The config is shared with the ssh-mesh project.
	sshd.SSHConfig `json:inline`


	// Most settings should go to 'mesh' and are common.
	// 'Dest' and identities configs are in MeshCfg.

	// Additional defaults for outgoing connections.
	// Probably belong to Dest.
	ConnectTimeout Duration `json:"connect_timeout,omitempty"`

	TCPUserTimeout time.Duration

	// Timeout used for TLS or SSH handshakes. If not set, 3 seconds is used.
	HandsahakeTimeout time.Duration

	// Configured hosts, key is a domain name without port.
	// This includes public keys, active addresses. Discovery and on-demand
	// are also used to load this info.
	// [namespace]/Node/[WorkloadID]
	// WorkloadID can be 32B SHA256(cert), 16 or 8B (VIP6) or 'trusted' IP (if infra is
	// secure - Wireguard or IPsec equivalent).

	// Clusters by hostname. The key is primarily a hostname:port, matching Istio/K8S Service name and ports.
	// TODO: do we need the port ? With ztunnel all endpoins can be reached, and the service selector applies
	// to all ports.
	//
	// Generally MeshClusters have different public keys/certs.
	// Includes Nodes, Pods and Services - the key can be the hash of the public key.
	Clusters map[string]*MeshCluster `json:clusters,omitempty"`


	// Internal ports

	// BasePort is the first port used for the virtual/control ports.
	// For Istio interop, it defaults to 15000 and uses same offsets.
	// This port is used for admin/debug/local MDS, bound to localhost, http protocol
	// Deprecated - use listeners
	BasePort int `json:"basePort,omitempty"`

}

type Duration struct {
	time.Duration
}

func (ms Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(ms.String())
}

func (ms *Duration) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*ms = Duration{Duration: time.Duration(value)}
		return nil
	case string:
		var err error
		s, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*ms = Duration{Duration: s}
		return nil
	default:
		return errors.New("invalid duration")
	}
}

// UGate represents a node using a HTTP/2 or HTTP/3 based overlay network environment.
// This can act as a minimal REST client and server - or can be used as a RoundTripper, Dialer and PortListener
// compatible with HBONE protocol and mesh security.
//
// UGate by default uses mTLS, using spiffee identities encoding K8S namespace, KSA and a trust
// domain. Other forms of authentication can be supported - auth is handled via configurable
// interface, not part of the core package.
//
// UGate can be used as a client, server or proxy/gateway.
type UGate struct {
	*MeshSettings

	// Auth plugs-in mTLS support. The generated configs should perform basic mesh
	// authentication.
	// Typically a *meshauth.MeshAuth
	Auth *meshauth.MeshAuth `json:"-"`

	// AuthProviders - matching kubeconfig user.authProvider.name
	// It is expected to return tokens with the given audience - in case of GCP
	// returns access tokens. If not set the cluster can't be created.
	//
	// A number of pre-defined token sources are used:
	// - gcp - returns GCP access tokens using MDS or default credentials. Used for example by GKE clusters.
	// - k8s - return K8S WorkloadID tokens with the given audience for default K8S cluster.
	// - istio-ca - returns K8S tokens with istio-ca audience - used by Citadel and default Istiod
	// - sts - federated google access tokens associated with GCP identity pools.
	AuthProviders map[string]func(context.Context, string) (string, error)

	// ReverseProxy is used when UGate is used to proxy to a local http/1.1 server.
	ReverseProxy *httputil.ReverseProxy

	// h2Server is the server used for accepting HBONE connections
	//h2Server *http2.Server
	// h2t is the transport used for all h2 connections used.
	// UGate is the connection pool, gets notified when con is closed.
	//h2t *http2.H2Transport

	// Main HTTP handler - will perform auth, dispatch, etc
	H2Handler http.Handler

	// Mux is used for HTTP and gRPC handler exposed externally.
	//
	// It is the handler for "hbone" and "hbonec" protocol handlers.
	//
	// The HTTP server on localhost:15000 uses http.DefaultMux - which is used by pprof
	// and others by default.
	Mux *http.ServeMux

	// MuxDialers are used to create an association with a peer and multiplex connections.
	// HBone, SSH, etc can act as mux dialers.
	MuxDialers map[string]meshauth.ContextDialer

	ListenerProto map[string]func(gate *UGate, l *meshauth.PortListener) error

	// EndpointResolver hooks into the Dial process and return the configured
	// EndpointCon object. This integrates with the XDS/config plane, with
	// additional local configs.
	//EndpointResolver func(sni string) *EndpointCon

	m sync.RWMutex

	Client *http.Client

	http1SChan chan net.Conn
	http1CChan chan net.Conn

	Http11Transport *http.Transport

	// Default dialer used to connect to host:port extracted from metadata.
	// Defaults to net.Dialer, making real connections.
	//
	// Can be replaced with a mux or egress dialer or router for
	// integration.
	NetDialer meshauth.ContextDialer

	// Used for udp proxy, when a captured packet is received.
	DNS        UDPHandler
	UDPHandler UDPHandler

	// Active connection by stream tuple, for MDS and debug.
	// This is primarily used for proxied connection, to allow the receiver to get metadata
	// (certs, real caller, etc)
	ActiveTcp map[string]nio.Stream

	TcpConActive *expvar.Int
	TcpConTotal  expvar.Int

	// template, used for TLS connections and the host WorkloadID
	TLSConfig *tls.Config

	StartFunctions []func(ug *UGate)
}

// Modules are used with conditional compiled modules, to reduce deps and binary size.
// The function will be called when the Gate is created - they may initialize.
// gate.StartFunctions will be called during Start().
var Modules = map[string]func(gate *UGate){}

// New creates a new UGate node. It requires a workload identity, including mTLS certificates.
func New(auth *meshauth.MeshAuth, ms *MeshSettings) *UGate {
	// For tests - main() and libraries should initialize the 3 configs.
	if ms == nil {
		ms = &MeshSettings{}
	}

	ug := &UGate{
		Auth: auth,
		MeshSettings:  ms,
		ListenerProto: map[string]func(gate *UGate, l *meshauth.PortListener) error{},
		Mux:           http.NewServeMux(),

		Client:       http.DefaultClient,
		NetDialer:    &net.Dialer{},
		MuxDialers:   map[string]meshauth.ContextDialer{},
		ActiveTcp:    map[string]nio.Stream{},
		TcpConActive: &expvar.Int{},
	}

	// Init default HTTP handler
	ug.H2Handler = &httpwrapper.HttpHandler{
		Handler: ug.Mux,
		Logger:  slog.With("id", ms.Name),
	}

	ug.Http11Transport = &http.Transport{
		DialContext: ug.DialContext,
		// If not set, DialContext and TLSClientConfig are used
		DialTLSContext:        ug.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		Proxy:                 http.ProxyFromEnvironment,
	}
	if ug.Auth == nil {
		ug.Auth = meshauth.NewMeshAuth(&ms.MeshCfg)
		if ug.Priv == "" {
			ug.Auth.InitSelfSigned("")
		}
	}

	if ms.Listeners == nil {
		ms.Listeners = map[string]*meshauth.PortListener{}
	}

	if ms.HandsahakeTimeout == 0 {
		ms.HandsahakeTimeout = 5 * time.Second
	}

	if ms.Clusters == nil {
		ms.Clusters = map[string]*MeshCluster{}
	//} else {
	//	for _, c := range ms.Clusters {
	//		c.UGate = ug
	//	}
	}

	if ms.ConnectTimeout.Duration == 0 {
		ms.ConnectTimeout.Duration = 5 * time.Second
	}

	// Init the HTTP reverse proxy, for apps listening for HTTP/1.1 on 8080
	// This is used for serverless but also support regular pods.
	// TODO: customize the port.
	// TODO: add a h2 reverse proxy as well on 8082, and grpc on 8081
	u, _ := url.Parse("http://127.0.0.1:8080")
	ug.ReverseProxy = httputil.NewSingleHostReverseProxy(u)

	msgs.InitMux(msgs.DefaultMux, ug.Mux, ug.Auth)

	ug.Mux.Handle("/debug/", http.DefaultServeMux)

	for _, m := range Modules {
		m(ug)
	}

	return ug
}


//// Handler is a handler for net.Conn with metadata.
//// Lighter alternative to http.Handler - used for TCP listeners.
//type Handler interface {
//	// HandleConn will process a received connection.
//	// TODO: add a ctx as soon as accept is called, including meta.
//	HandleConn(net.Conn) error
//}
//
//// Wrap a function as a stream handler.
//type HandlerFunc func(conn net.Conn) error
//
//func (c HandlerFunc) HandleConn(conn net.Conn) error {
//	return c(conn)
//}

// UDPHandler is used to abstract the handling of incoming UDP packets on a UDP
// listener or TUN.
type UDPHandler interface {
	HandleUdp(dstAddr net.IP, dstPort uint16, localAddr net.IP, localPort uint16, data []byte)
}

// UdpWriter is the interface implemented by the TunTransport, to send
// packets back to the virtual interface. TUN or TProxy raw support this.
// Required for 'transparent' capture of UDP - otherwise use STUN/TURN/etc.
// A UDP NAT does not need this interface.
type UdpWriter interface {
	WriteTo(data []byte, dstAddr *net.UDPAddr, srcAddr *net.UDPAddr) (int, error)
}


// All streams must call this method once a connection is made, and defer OnStreamDone
func (ug *UGate) OnStream(s nio.Stream) {
	ug.TcpConActive.Add(1)
	ug.TcpConTotal.Add(1)
}

// Called at the end of the connection handling. After this point
// nothing should use or refer to the connection, both proxy directions
// should already be closed for write or fully closed.
func (ug *UGate) OnStreamDone(str nio.Stream) {

	ug.m.Lock()
	delete(ug.ActiveTcp, str.State().StreamId)
	ug.m.Unlock()
	ug.TcpConActive.Add(-1)
	// TODO: track multiplexed streams separately.
	//if str.ReadErr != nil {
	//	VarzSErrRead.Add(1)
	//}
	//if str.WriteErr != nil {
	//	VarzSErrWrite.Add(1)
	//}
	//if str.ProxyReadErr != nil {
	//	VarzCErrRead.Add(1)
	//}
	//if str.ProxyWriteErr != nil {
	//	VarzCErrWrite.Add(1)
	//}

	if r := recover(); r != nil {

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
		log.Println("AC: Recovered in f", r, err)
	}

	//if NoEOF(str.ReadErr) != nil || str.WriteErr != nil {
	//	log.Println(str.StreamId, "AE:", "Err in:", str.ReadErr, str.WriteErr)
	//}
	//if NoEOF(str.ProxyReadErr) != nil || str.ProxyWriteErr != nil {
	//	log.Println(str.StreamId, "AE:", "Err out:", str.ProxyReadErr, str.ProxyWriteErr)
	//}
	//if !str.Closed {
	//	str.Close()
	//}

	ug.OnSClose(str, str.RemoteAddr())
}

func (gw *UGate) OnMuxClose(dm *MeshCluster) {

}

func (gw *UGate) OnSClose(s nio.Stream, addr net.Addr) {
	//str := s.State()
	//if str.ReadErr != nil || str.WriteErr != nil {
	//	log.Printf("%d AC: src=%s://%v dst=%s rcv=%d/%d snd=%d/%d la=%v ra=%v op=%v %v %v",
	//		str.StreamId,
	//		str.Type, addr,
	//		str.Dest,
	//		str.RcvdPackets, str.RcvdBytes,
	//		str.SentPackets, str.SentBytes,
	//		time.Since(str.LastWrite),
	//		time.Since(str.LastRead),
	//		int64(time.Since(str.Open).Seconds()),
	//		str.ReadErr, str.WriteErr)
	//	return
	//}
	//log.Printf("AC: %d src=%s://%v dst=%s rcv=%d/%d snd=%d/%d la=%v ra=%v op=%v",
	//	str.StreamId,
	//	str.Type, addr,
	//	str.Dest,
	//	str.RcvdPackets, str.RcvdBytes,
	//	str.SentPackets, str.SentBytes,
	//	time.Since(str.LastWrite),
	//	time.Since(str.LastRead),
	//	int64(time.Since(str.Open).Seconds()))
}

func (ug *UGate) RegisterProxyStream(s nio.Stream) {
	// TODO: compute StreamId based on meta ( tuple )
	ug.m.Lock()
	ug.ActiveTcp[s.State().StreamId] = s
	ug.m.Unlock()
}

// RemoteID returns the node WorkloadID based on authentication.
func (gw *UGate) RemoteID(s nio.Stream) string {
	tls := s.TLSConnectionState()
	if tls != nil {
		if len(tls.PeerCertificates) == 0 {
			return ""
		}
		pk, err := meshauth.VerifySelfSigned(tls.PeerCertificates)
		if err != nil {
			return ""
		}

		return meshauth.PublicKeyBase32SHA(pk)
	}
	return ""
}

// StartBHoneD will listen on addr as H2C (typically :15009)
//
//
// Incoming streams for /_hbone/mtls will be treated as a mTLS connection,
// using the Istio certificates and root. After handling mTLS, the clear text
// connection will be forwarded to localhost:8080 ( TODO: custom port ).
//
// TODO: setting for app protocol=h2, http, tcp - initial impl uses tcp
//
// Incoming requests for /_hbone/22 will be forwarded to localhost:22, for
// debugging with ssh.
//


// HandleTCPProxy connects and forwards r/w to the hostPort
func (hb *UGate) HandleTCPProxy(w io.Writer, r io.Reader, hostPort string) error {
	log.Println("net.RoundTripStart", hostPort)
	nc, err := net.Dial("tcp", hostPort)
	if err != nil {
		log.Println("Error dialing ", hostPort, err)
		return err
	}

	return nio.Proxy(nc, r, w, hostPort)
}

// HttpClient returns a http.Client configured with the specified root CA, and reasonable settings.
// The URest wrapper is added, for telemetry or other interceptors.
func (hb *UGate) HttpClient(caCert []byte) *http.Client {
	// The 'max idle conns, idle con timeout, etc are shorter - this is meant for
	// fast initial config, not as a general purpose client.
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,

		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
	}

	if caCert != nil && len(caCert) > 0 {
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(caCert) {
			log.Println("Failed to decode PEM")
		}
		tr.TLSClientConfig = &tls.Config{
			RootCAs: roots,
		}
	}

	return &http.Client{
		Transport: tr,
	}
}


// Dealing with capture

// HanldeTUN is called when a TCP egress connection is intercepted via TProxy or TUN (gVisor or lwip)
// target is the destination address, la is the local address (the connection will have it reversed).
func (hb *UGate) HandleTUN(nc net.Conn, target *net.TCPAddr, la *net.TCPAddr) {
	log.Println("TUN TCP ", target, la)
	dest := target.String()

	rc, err := hb.Dial("tcp", dest)
	if err != nil {
		nc.Close()
		return
	}
	nio.Proxy(rc, nc, nc, dest)
	return
}

// HandleUdp is the common entry point for UDP capture.
// - tproxy
// - gvisor/lwIP
// WIP
func (hb *UGate) HandleUdp(dstAddr net.IP, dstPort uint16,
	localAddr net.IP, localPort uint16,
	data []byte) {
	log.Println("TProxy UDP ", dstAddr, dstPort, localAddr, localPort, len(data))
}

type Route struct {
	// Addr (ex :8080). This is the requested address.
	//
	// BTS, SOCKS, HTTP_PROXY and IPTABLES have default ports and bindings, don't
	// need to be configured here.
	//Addr string `json:"address,omitempty"`

	// How to connect. Default: original dst
	//Protocol string `json:"proto,omitempty"`
}



// Start listening on all configured ports.
// This doesn't have to be called if ugate is used in client mode.
func (ug *UGate) Start() error {
	// Explicit TCP forwarders.
	for k, t := range ug.Listeners {
		if t.Name == "" {
			t.Name = k
		}
		err := ug.StartListener(t)
		if err != nil {
			slog.Warn("Failed to start listener", "name", t.Port, "err", err)
		} else {
			slog.Info("listener", "addr", t.Address, "proto", t.Protocol)
		}
	}

	return nil
}

func (ug *UGate) Close() error {
	var err error
	for _, p := range ug.Listeners {
		if p.NetListener != nil {
			e := p.NetListener.Close()
			if e != nil {
				err = err
			}
			p.NetListener = nil
		}
	}
	return err
}

// OnHClose called on http close
func (gw *UGate) OnHClose(s string, id string, san string, r *http.Request, since time.Duration) {
	slog.Info("HTTP", "method", r.Method,
		"url", r.URL, "proto", r.Proto, "header", r.Header, "id", id,
		"san", san, "remote", r.RemoteAddr, "d", since)
}

var LogClose = true

// StartListener and Start a real port listener on a port.
// Virtual listeners can be added to ug.Conf or the mux.
// Creates a raw (port) TCP listener. Accepts connections
// on a local port, forwards to a remote destination.
func (ug *UGate) StartListener(ll *meshauth.PortListener) error {
	if ll.Protocol == "" {
		parts := strings.Split(ll.Name, "-")
		ll.Protocol = parts[0]
		if ll.Address == "" && ll.Port == 0 {
			if len(parts) == 2 {
				p, _ := strconv.Atoi(parts[1])
				ll.Port = int32(p)
			} else if len(parts) > 2 {
				ll.Address = net.JoinHostPort(parts[1], parts[2])
			}
		}
	}

	if ll.Address == "" && ll.Port != 0 {
		ll.Address = fmt.Sprintf(":%d", ll.Port)
	}


	f := ug.ListenerProto[ll.Protocol]
	if f != nil {
		go func() {
			err := f(ug, ll)
			if err != nil {
				slog.Info("Listener error", "err", err, "addr", ll.Address, "name", ll.Name, "proto", ll.Protocol)
			}
		}()
		return nil
	}

	slog.Info("Listener error", "err", "missing protocol handler", "addr", ll.Address, "name", ll.Name, "proto", ll.Protocol)
	return errors.New("Missing handler" + ll.Protocol)
}


// DialTLS dials a TLS connection to addr and does the handshake.
// It opens a direct TLS connection using the dialer for TCP.
// No peer verification - the returned stream will have the certs.
// addr is a real internet address, not a mesh one.
//
// Used internally to create the raw TLS connections to both mesh
// and non-mesh nodes.
// Do a TLS handshake on the plain text nc.
// Verify the server identity using a remotePeerID - based on public key.
// TODO: add syncthing style hash of cert, spiffee, DNS as alternative identities.
// TODO: add root CAs (including public) and SHA of root cert.
func (*UGate) NewTLSConnOut(ctx context.Context, nc net.Conn,
	cfg *meshauth.MeshAuth,
	remotePeerID string, alpn []string) (nio.Stream, error) {

	tlsc, err := cfg.TLSClient(ctx, nc, &meshauth.Dest{
		Addr:          "",
		CACertPEM:     nil,
		TokenProvider: nil,
		TokenSource:   "",
		SNI:           remotePeerID,
		ALPN:          alpn,
	},   remotePeerID)
	if err != nil {
		return nil, err
	}
	tlsS := tlsc.ConnectionState()

	s := nio.NewStreamConn(tlsc)
	s.TLS = &tlsS
	//s.State().PeerPublicKey = remotePubKey

	return s, nil
}

// DialMUX creates an association with the node, using one of the supported
// transports.
//
// The node should have at least the address or public key or hash populated.
func (ug *UGate) DialMUX(ctx context.Context, net string, node *MeshCluster, ev func(t string, stream nio.Stream)) (http.RoundTripper, error) {
	if node.RoundTripper != nil {
		return node.RoundTripper, nil
	}

	return node.HttpClient().Transport, nil
}

// DialContext dials a destination address (host:port).
// This can be used in applications as a TCP Dial replacement.
//
// It will first attempt to look up the host config, and if it supports 'mesh' will
// use a secure, multiplexed connection.
//
func (hb *UGate) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	c := hb.GetCluster(addr)
	if c == nil {
		host, _, _ := net.SplitHostPort(addr)
		c = hb.GetCluster(host)
	}


	if c!= nil {

		if c.Dialer == nil && c.Proto != ""  {
			// TODO: set proto based on labels
			c.Dialer = hb.MuxDialers[c.Proto]
		}

		if c.Dialer != nil {
			return c.Dialer.DialContext(ctx, network, addr)
		} else {
			log.Println("Invalid dialer")
			return nil, errors.New("Missing dialer for protocol")
		}


		// TODO: routing, etc - based on endpoints and TcpRoutes
	}

	// TODO: if egress gateway is set, use it ( redirect all unknown to egress )
	// TODO: CIDR range of Endpoints, Nodes, VIPs to use hbone
	// TODO: if port, use SNI or match clusters
	nc, err := hb.NetDialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	// If net connection is cut, by default the socket may linger for up to 20 min without detecting this.
	// Extracted from gRPC - needs to apply at TCP socket level
	if c.TCPUserTimeout != 0 {
		syscall.SetTCPUserTimeout(nc, c.TCPUserTimeout)
	}

	return nc, err
}

// RoundTrip makes a HTTP request (over some secure transport including ambient and tunnels), over
// a multiplexed or direct connection.
//func (ug *UGate) RoundTrip(req *http.Request) (*http.Response, error) {
//	hostPort := req.Host
//	if hostPort == "" {
//		hostPort = req.URL.Host
//	}
//	c, err := ug.Cluster(req.Context(), hostPort)
//	if err != nil {
//		return nil, err
//	}
//
//	return c.RoundTrip(req)
//}



// Dial calls @See DialContext
func (hb *UGate) Dial(n, a string) (net.Conn, error) {
	return hb.DialContext(context.Background(), n, a)
}

