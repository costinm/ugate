package ugate

import (
	"context"
	"encoding/json"
	"expvar"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

var (
	StreamId = uint32(0)
)

var (
	VarzAccepted  = expvar.NewInt("ugate.accepted")
	VarzAcceptErr = expvar.NewInt("ugate.acceptErr")

	// Proxy behavior
	VarzReadFrom  = expvar.NewInt("ugate.sReadFrom")
	VarzReadFromC = expvar.NewInt("ugate.cReadFrom")

	VarzSErrRead  = expvar.NewInt("ugate.sErrRead")
	VarzSErrWrite = expvar.NewInt("ugate.sErrWrite")
	VarzCErrRead  = expvar.NewInt("ugate.cErrRead")
	VarzCErrWrite = expvar.NewInt("ugate.cErrWrite")

	VarzMaxRead = expvar.NewInt("ugate.maxRead")

	// Managed by 'NewTCPProxy' - before dial.
	TcpConTotal = expvar.NewInt("gate:tcp:total")

	// Managed by updateStatsOnClose - including error cases.
	TcpConActive = expvar.NewInt("gate:tcp:active")
)

// Configuration for the micro Gateway.
//
type GateCfg struct {
	// BasePort is the first port used for the virtual/control ports.
	// For Istio interop, it defaults to 15000 and uses same offsets.
	// Primarily used for testing, or if Istio is also used.
	BasePort int `json:"basePort,omitempty"`

	// Name of the node, defaults to hostname or POD_NAME
	Name string `json:"name,omitempty"`

	// Domain name, also default control plane and gateway.
	Domain string `json:"domain,omitempty"`

	// Additional port listeners.
	// Egress: listen on 127.0.0.1:port
	// Ingress: listen on 0.0.0.0:port (or actual IP)
	//
	// Port proxies: will register a listener for each port, forwarding to the
	// given address.
	Listeners map[string]*Listener `json:"listeners,omitempty"`

	// Configured hosts, key is a domain name without port.
	Hosts map[string]*DMNode `json:"hosts,omitempty"`

	// Egress gateways for reverse H2 - key is the domain name, value is the
	// pubkey or root CA. If empty, public certificates are required.
	H2R map[string]string `json:"remoteAccept,omitempty"`
}

const (
	// Offsets from BasePort for the default ports.

	// Used for local UDP messages and multicast.
	PORT_UDP = 8

	PORT_IPTABLES = 1
	PORT_IPTABLES_IN = 6
	PORT_SOCKS = 9
	// SNI and HTTP could share the same port - would also
	// reduce missconfig risks
	PORT_HTTP_PROXY = 2
	PORT_SNI = 3

	// H2, HTTPS, H2R
	PORT_HTTPS = 7
)

// Keyed by Hostname:port (if found in dns tables) or IP:port
type HostStats struct {
	// First open
	Open time.Time

	// Last usage
	Last time.Time

	SentBytes   int
	RcvdBytes   int
	SentPackets int
	RcvdPackets int
	Count       int

	LastLatency time.Duration
	LastBPS     int
}

// Node information, based on registration info or discovery.
// Used for 'mesh' nodes, where we have a public key and other info.
type DMNode struct {
	// Primary/main address and port of the BTS endpoint.
	// Can be a DNS name that resolves, or some other node.
	// Individual IPs (relay, etc) will be in the info.addrs field
	Addr string `json:"addr,omitempty"`

	// VIP is the mesh specific IP6 address. The 'network' identifies the master node, the
	// link part is the sha of the public key. This is a byte[16].
	// If not known, will be populated after the connection.
	// If set, will be verified and used to locate the node.
	VIP net.IP `json:"vip,omitempty"`

	// Public key.
	// If not known, will be populated after the connection.
	// If set, will be verified and used to locate the node.
	PublicKey []byte `json:"pub,omitempty"`

	// Information from the node - from an announce or message.
	NodeAnnounce *NodeAnnounce `json:"info,omitempty"`

	Labels map[string]string `json:"l,omitempty"`

	Bacokff time.Duration `json:"-"`

	// Last LL GW address used by the peer.
	// Public IP addresses are stored in Reg.IPs.
	// If set, returned as the first address in GWs, which is used to connect.
	// This is not sent in the registration - but extracted from the request
	// remote address.
	GW *net.UDPAddr `json:"gw,omitempty"`

	// IP4 address of last announce
	Last4 *net.UDPAddr `json:"-"`

	// IP6 address of last announce
	Last6 *net.UDPAddr `json:"-"`

	FirstSeen time.Time

	// Last packet or registration from the peer.
	LastSeen time.Time `json:"t"`

	// In seconds since first seen, last 100
	Seen []int `json:"-"`

	// LastSeen in a multicast announce
	LastSeen4 time.Time

	// LastSeen in a multicast announce
	LastSeen6 time.Time `json:"-"`

	// Number of multicast received
	Announces int `json:"-"`

	// Numbers of announces received from that node on the P2P interface
	AnnouncesOnP2P int `json:"-"`

	// Numbers of announces received from that node on the P2P interface
	AnnouncesFromP2P int `json:"-"`

	// H2r is a HTTP2 connection to the node.
	// May be a direct or reverse connection.
	H2r Muxer `json:"-"`

	// Set if the gateway has an active incoming connection from this
	// node, with the node acting as client.
	// Streams will be forwarded to the node using special 'accept' mode.
	// This is similar with PUSH in H2.
	// Deprecated - impl in ssh server, use in old wpgate DialMeshLocal
	TunSrv MuxedConn `json:"-"`

	// Existing tun to the remote node, previously dialed.
	// Deprecated
	TunClient MuxedConn `json:"-"`
}

// Muxer is the interface implemented by a multiplexed connection with metadata
// http2.ClientConn is the default implementation used.
type Muxer interface {
	http.RoundTripper

	io.Closer

	Ping(ctx context.Context) error
}

const ProtoTLS = "tls"

// autodetected for TLS.
const ProtoH2 = "h2"
const ProtoHTTP = "http" // 1.1
const ProtoHTTPS = "https"

const ProtoHAProxy = "haproxy"

// Egress capture - for dedicated listeners
const ProtoSocks = "socks5"
const ProtoIPTables = "iptables"
const ProtoIPTablesIn = "iptables-in"
const ProtoConnect = "connect"

// Listener represents the configuration for an acceptor on a port.
//
// For each port, metadata is optionally extracted - in particular a 'hostname', which
// is the destination IP or name:
// - socks5 dest
// - iptables original dst ( may be combined with DNS interception )
// - NAT dst address
// - SNI for TLS
// - :host header for HTTP
//
// Stream Dest and meta will be set after metadata extraction.
//
// Based on Istio/K8S Gateway models.
type Listener struct {

	// Address address (ex :8080). This is the requested address.
	// Addr() returns the actual address of the listener.
	Address string `json:"address,omitempty"`

	// Port can have multiple protocols:
	// - iptables
	// - iptables_in
	// - socks5
	// - tls - will use SNI to detect the host config, depending
	// on that we may terminate or proxy
	// - http - will auto-detect http/2, proxy
	//
	// If missing or other value, this is a dedicated port, specific to a single
	// destination.
	Protocol string `json:"proto,omitempty"`

	// ForwardTo where to forward the proxied connections.
	// Used for accepting on a dedicated port. Will be set as Dest in
	// the stream, can be mesh node.
	// IP:port format, where IP can be a mesh VIP
	ForwardTo string `json:"forwardTo,omitempty"`

	// Custom listener - not guaranteed to return TCPConn.
	// After the listener is added, will be set to the port listener.
	// For future use with IPFS
	NetListener net.Listener `json:-`

	// Must block until the connection is fully handled !
	Handler ConHandler `json:-`

//	gate *UGate `json:-`
}

// Mapping to Istio:
// - gateway port -> listener conf
// - Remote -> shortcut for a TCP listener with  single deset.
// - SNI/Socks -> host extracted from protocol.
//
//

// -------------------- Used by upgate

// TODO: use net.Dialer (timeout, keep alive, resolver, etc)

// TODO: use net.Dialer.DialContext(ctx context.Context, network, address string) (Conn, error)
// Dialer also uses nettrace in context, calls resolver,
// can do parallel or serial calls. Supports TCP, UDP, Unix, IP

// nettrace: internal, uses TraceKey in context used for httptrace,
// but not exposed. Net has hooks into it.
// Dial, lookup keep track of DNSStart, DNSDone, ConnectStart, ConnectDone

// httptrace:
// WithClientTrace(ctx, trace) Context
// ContextClientTrace(ctx) -> *ClientTrace

// Defined in x.net.proxy
// Used to create the actual connection to an address.
type ContextDialer interface {
	DialContext(ctx context.Context, net, addr string) (net.Conn, error)
}

// Alternative to http.Handler
type ConHandler interface {
	Handle(conn MetaConn) error
}

type ConHandlerF func(conn MetaConn) error

func (c ConHandlerF) Handle(conn MetaConn) error {
	return c(conn)
}

// For integration with TUN
// TODO: use same interfaces.

// UdpWriter is the interface implemented by the TunTransport, to send
// packets back to the virtual interface. TUN or TProxy raw support this.
// Required for 'transparent' capture of UDP - otherwise use STUN/TURN/etc.
// A UDP NAT does not need this interface.
type UdpWriter interface {
	WriteTo(data []byte, dstAddr *net.UDPAddr, srcAddr *net.UDPAddr) (int, error)
}

type HostResolver interface {
	// HostByAddr returns the last lookup address for an IP, or the original
	// address. The IP is expressed as a string ( ip.String() ).
	HostByAddr(addr string) (string, bool)
}

// Interface implemented by TCPHandler.
type UDPHandler interface {
	HandleUdp(dstAddr net.IP, dstPort uint16, localAddr net.IP, localPort uint16, data []byte)
}

// Used by the TUN interface.
type TCPHandler interface {
	HandleTUN(conn net.Conn, target *net.TCPAddr) error
}


// IPFS:
// http://<gateway host>/ipfs/CID/path
// http://<cid>.ipfs.<gateway host>/<path>
// http://gateway/ipns/IPNDS_ID/path
// ipfs://<CID>/<path>, ipns://<peer ID>/<path>, and dweb://<IPFS address>
//
// Multiaddr: TLV

// Internal use.

type MetaConn interface {
	net.Conn
	//http.ResponseWriter

	// Returns request metadata. This is a subset of
	// http.Request. An adapter exists to convert this into
	// a proper http.Request and use normal http.Handler and
	// RoundTrip.
	Meta() *Stream
}
type CloseWriter interface {
	CloseWrite() error
}

type CloseReader interface {
	CloseRead() error
}



// Helpers for very simple configuration and key loading.
// Will use json files for config.
type ConfStore interface {
	// Get a config blob by name
	Get(name string) ([]byte, error)

	// Save a config blob
	Set(conf string, data []byte) error

	// List the configs starting with a prefix, of a given type
	List(name string, tp string) ([]string, error)
}

// ConfInt returns an string setting, with default value, from the ConfStore.
func ConfStr(cs ConfStore, name, def string) string {
	if cs == nil {
		return def
	}
	b, _ := cs.Get(name)
	if b == nil {
		return def
	}
	return string(b)
}

// ConfInt returns an int setting, with default value, from the ConfStore.
func ConfInt(cs ConfStore, name string, def int) int {
	if cs == nil {
		return def
	}
	b, _ := cs.Get(name)
	if b == nil {
		return def
	}
	v, err := strconv.Atoi(string(b))
	if err != nil {
		return def
	}
	return v
}

// IPResolver uses DNS cache or lookups to return the name
// associated with an IP, for metrics/stats/logs
type IPResolver interface {
	IPResolve(ip string) string
}

// Information about a node.
// Sent periodically, signed by the origin - for example as a JWT, or UDP
// proto.
// TODO: map it to Pod, IPFS announce
// TODO: move Wifi discovery to separate package.
type NodeAnnounce struct {
	UA string `json:"UA,omitempty"`

	// Non-link local IPs from all interfaces. Includes public internet addresses
	// and Wifi IP4 address. Used to determine if a node is directly connected.
	IPs []*net.UDPAddr `json:"addrs,omitempty"`

	// Set if the node is an active Android AP.
	SSID string `json:"ssid,omitempty"`

	// True if the node is an active Android AP on the interface sending the message.
	// Will trigger special handling in link-local - if the receiving interface is also
	// an android client.
	AP bool `json:"AP,omitempty"`

	Ack bool `json:"ACK,omitempty"`

	// VIP of the direct parent, if this node is connected.
	// Used to determine the mesh topology.
	Vpn string `json:"Vpn,omitempty"`
}

// Transport is creates multiplexed connections.
//
// On the server side, MuxedConn are created when a client connects.
type Transport interface {
	// Dial one TCP/mux connection to the IP:port.
	// The destination is a mesh node - port typically 5222, or 22 for 'regular' SSH serves.
	//
	// After handshake, an initial message is sent, including informations about the current node.
	//
	// The remote can be a trusted VPN, an untrusted AP/Gateway, a peer (link local or with public IP),
	// or a child. The subsriptions are used to indicate what messages will be forwarded to the server.
	// Typically VPN will receive all events, AP will receive subset of events related to topology while
	// child/peer only receive directed messages.
	DialMUX(addr string, pub []byte, subs []string) (MuxedConn, error)
}

// A Connection that can multiplex.
// Will dial a stream, may also accept streams and dispatch them.
//
// For example SSHClient, SSHServer, Quic can support this.
type MuxedConn interface {
	// DialProxy will use the remote gateway to jump to
	// a different destination, indicated by stream.
	// On return, the stream ServerOut and ServerIn will be
	// populated, and connected to stream Dest.
	// deprecated:  use CreateStream
	DialProxy(tp *Stream) error

	// The VIP of the remote host, after authentication.
	RemoteVIP() net.IP

	// Wait for the conn to finish.
	Wait() error
}

type StreamCreator interface {
	CreateStream(ctx context.Context, n *DMNode, r1 *http.Request) (*Stream, error)
}

// Textual representation of the node registration data.
func (n *DMNode) String() string {
	b, _ := json.Marshal(n)
	return string(b)
}

// Return the list of gateways for the node, starting with the link local if any.
func (n *DMNode) GWs() []*net.UDPAddr {
	res := []*net.UDPAddr{}

	if n.GW != nil {
		res = append(res, n.GW)
	}
	if n.Last4 != nil {
		res = append(res, n.Last4)
	}
	if n.Last6 != nil {
		res = append(res, n.Last6)
	}
	return res
}

// Called when receiving a registration or regular valid message via a different gateway.
// - HandleRegistrationRequest - after validating the VIP
//
//
// For VPN, the srcPort is assigned by the NAT, can be anything
// For direct, the port will be 5228 or 5229
func (n *DMNode) UpdateGWDirect(addr net.IP, zone string, srcPort int, onRes bool) {
	n.LastSeen = time.Now()
	n.GW = &net.UDPAddr{IP: addr, Port: srcPort, Zone: zone}
}
func (n *DMNode) BackoffReset() {
	n.Bacokff = 0
}
func (n *DMNode) BackoffSleep() {
	if n.Bacokff == 0 {
		n.Bacokff = 5 * time.Second
	}
	time.Sleep(n.Bacokff)
	if n.Bacokff < 5*time.Minute {
		n.Bacokff = n.Bacokff * 2
	}
}

// === Messaging
// The former messaging interface is mapped to HTTP:
// - incoming messages are treated as HTTP requests. CloudEvents/Pubub/etc are ok.
// - polling results in posting same HTTP requests, internally
// - sending events: using a special HttpClient.
//
// Main 'difference' between a message and a regular HTTP is the size of request is limited.
// Webpush is also mapped in the same way - the glue code handles encryption/decryption.
// PubSubMessage is the payload of a Pub/Sub event.
type PubSubMessage struct {
	Message struct {
		Data []byte `json:"data,omitempty"`
		ID   string `json:"id"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}
