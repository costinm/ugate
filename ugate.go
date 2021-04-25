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
	// If not set, the test/open domain is used, expect poor behavior
	// (c1.webinf.info). If set to "-", no parent control plane.
	// This also acts as 'trust domain' and 'mesh id'.
	Domain string `json:"domain,omitempty"`

	// TODO: other trusted domains/meshes (federation, migration, etc)
	//DomainAliases []string `json:"domainAliases,omitempty"`

	// Additional port listeners.
	// Egress: listen on 127.0.0.1:port
	// Ingress: listen on 0.0.0.0:port (or actual IP)
	//
	// Port proxies: will register a listener for each port, forwarding to the
	// given address.
	Listeners map[string]*Listener `json:"listeners,omitempty"`

	// Configured hosts, key is a domain name without port.
	// This includes public keys, active addresses. Discovery and on-demand
	// are also used to load this info.
	// TODO: move to separate files:
	// [namespace]/Node/[ID]
	// ID can be 32B SHA256(cert), 16 or 8B (VIP6) or 'trusted' IP (if infra is
	// secure - Wireguard or IPsec equivalent).
	Hosts map[string]*DMNode `json:"hosts,omitempty"`

	// Egress gateways for reverse H2 - key is the domain name, value is the
	// pubkey or root CA. If empty, public certificates are required.
	H2R  map[string]string `json:"remoteAccept,omitempty"`

	// ALPN to announce on the main BTS port
	ALPN []string
}

const (
	// Offsets from BasePort for the default ports.

	PORT_IPTABLES = 1
	PORT_IPTABLES_IN = 6
	PORT_SOCKS = 5
	// SNI and HTTP could share the same port - would also
	// reduce missconfig risks
	PORT_HTTP_PROXY = 2
	PORT_SNI = 3

	// TLS/SNI, HTTPS (no client certs)
	PORT_HTTPS = 4

	// H2, HTTPS, H2R
	PORT_BTS = 7
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
//
// Used for 'mesh' nodes, where we have a public key and other info, as well
// as non-mesh nodes.
//
// This struct includes statistics about the node and current active association/mux
// connections.
//
type DMNode struct {
	// ID is the (best) primary id known for the node. Format is:
	//    base32(SHA256(EC_256_pub)) - 32 bytes binary, 52 bytes encoded
	//    base32(ED_pub) - same size, for nodes with ED keys.
	//
	// For non-mesh nodes, it is a (real) domain name or IP if unknown.
	// It may include port, or even be a URL - the external destinations may
	// have different public keys on different ports.
	//
	// The node may be a virtual IP ( ex. K8S/Istio service ) or name
	// of a virtual service.
	//
	// If IPs are used, they must be either truncated SHA or included
	// in the node cert or the control plane must return metadata and
	// secure low-level network is used (like wireguard)
	//
	// Required for secure communication.
	//
	// Examples:
	//  -  [B32_SHA]
	//  -  [B32_SHA].reviews.bookinfo.svc.example.com
	//  -  IP6 (based on SHA or 'trusted' IP)
	//  -  IP4 ('trusted' IP)
	// TODO: To support captured traffic, the IP6 format is also supported.
	//
	ID string `json:"id,omitempty"`

	// IDAlias is a list of alternate identities associated with the
	// node.
	//
	// TODO: implement
	IDAlias []string `json:"alias,omitempty"`

	// Groups is a list of groups and roles the node is associated with.
	//
	Groups []string `json:"groups,omitempty"`

	// TODO: rename to node ID - use truncated or full form.

	// TODO: print Hex form as well, make sure the 8 bytes of the VIP are visible

	// Primary/main address and port of the BTS endpoint.
	// Can be a DNS name that resolves, or some other node.
	// Individual IPs (relay, etc) will be in the info.addrs field
	// May also be a URL (webpush endpoint).
	Addr string `json:"addr,omitempty"`

	// Alternate addresses, list of URLs
	URLs []string `json:"urls,omitempty"`

	// Primary public key of the node.
	// EC256: 65 bytes, uncompressed format
	// RSA: DER
	// ED25519: 32B
	// Used for sending encryted webpush message
	// If not known, will be populated after the connection.
	PublicKey []byte `json:"pub,omitempty"`

	// Auth for webpush. A shared secret known by uGate and remote
	// node.
	Auth []byte `json:"auth,omitempty"`

	// Information from the node - from an announce or message.
	NodeAnnounce *NodeAnnounce `json:"info,omitempty"`

	Labels map[string]string `json:"l,omitempty"`

	// Will be set if there are problems connecting to the node
	// (or if connection duration is too short)
	Bacokff time.Duration `json:"-"`

	// IP4 address of last announce (link local) or connection
	Last4 *net.UDPAddr `json:"-"`
	// LastSeen in a multicast announce
	LastSeen4 time.Time

	// IP6 address of last announce or connection.
	Last6 *net.UDPAddr `json:"-"`
	// LastSeen in a multicast announce
	LastSeen6 time.Time `json:"-"`

	FirstSeen time.Time

	// Last packet or registration from the peer.
	LastSeen time.Time `json:"t"`

	// In seconds since first seen, last 100
	Seen []int `json:"-"`


	Stats *HostStats

	// Muxer is a HTTP2-like connection to the node.
	// Implements RoundTrip, with the semantics of CONNECT (no buffering)
	// May be a direct or reverse connection.
	Muxer Muxer `json:"-"`


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

	// ALPN to announce
	ALPN []string
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

// ContextDialer is same with x.net.proxy.ContextDialer
// Used to create the actual connection to an address using the mesh.
// The result may have metadata, and be an instance of ugate.Stream.
//
// A uGate implements this interface, it is the primary interface
// for creating streams where the caller does not want to pass custom
// metadata. Based on net and addr and handshake, if destination is
// capable we will upgrade to BTS and pass metadata. This may also
// be sent via an egress gateway.
//
// For compatibility, 'net' can be "tcp" and addr a mangled hostname:port
// Mesh addresses can be identified by the hostname or IP6 address.
// External addresses will create direct connections if possible, or
// use egress server.
//
// TODO: also support 'url' scheme
type ContextDialer interface {
	DialContext(ctx context.Context, net, addr string) (net.Conn, error)
}

// Muxer is the interface implemented by a multiplexed connection with metadata
// http2.ClientConn is the default implementation used.
type Muxer interface {
	http.RoundTripper

	io.Closer

	//Ping(ctx context.Context) error
}

type MuxDialer interface {
	// DialMux creates a bi-directional multiplexed association with the node.
	// The node must support a multiplexing protocol - the fallback is H2.
	//
	// Fallback:
	// For non-mesh nodes the H2 connection may not allow incoming streams or
	// messages. Mesh nodes emulate incoming streams using /h2r/ and send/receive
	// messages using /.dm/msg/
	DialMux(ctx context.Context, node *DMNode, meta http.Header, ev func(t string, stream *Stream)) (Muxer, error)
}

// StreamDialer is similar with RoundTrip, makes a single connection using a MUX.
//
// Unlike ContextDialer, also takes 'meta' and returns a Stream ( which implements net.Conn).
//
// UGate implements ContextDialer, so it can be used in other apps as a library without
// dependencies to the API. The context can be used for passing metadata.
// It also implements RoundTripper, since streams are mapped to HTTP.
//type StreamDialer interface {
//	DialStream(ctx context.Context, netw string, addr string, meta http.Header) (*Stream, error)
//}


// ConHandler is a handler for net.Conn with metadata.
// Lighter alternative to http.Handler
type ConHandler interface {
	Handle(conn MetaConn) error
}


type ConHandlerF func(conn MetaConn) error

func (c ConHandlerF) Handle(conn MetaConn) error {
	return c(conn)
}

// For integration with TUN
// TODO: use same interfaces.


type HostResolver interface {
	// HostByAddr returns the last lookup address for an IP, or the original
	// address. The IP is expressed as a string ( ip.String() ).
	HostByAddr(addr string) (string, bool)
}

// Used by the TUN interface
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


// IPFS:
// http://<gateway host>/ipfs/CID/path
// http://<cid>.ipfs.<gateway host>/<path>
// http://gateway/ipns/IPNDS_ID/path
// ipfs://<CID>/<path>, ipns://<peer ID>/<path>, and dweb://<IPFS address>
//
// Multiaddr: TLV

// Internal use.

// Deprecated
type MetaConn interface {
	net.Conn
	Meta() *Stream
}

type CloseWriter interface {
	CloseWrite() error
}

type CloseReader interface {
	CloseRead() error
}



// Interface for very simple configuration and key loading.
// Can have a simple in-memory, fs implementation, as well
// as K8S, XDS or database backends.
//
// The name is hierachical, in case of K8S or Istio corresponds
// to the type, including namespace.
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

//// Transport is creates multiplexed connections.
////
//// On the server side, MuxedConn are created when a client connects.
//type Transport interface {
//	// Dial one TCP/mux connection to the IP:port.
//	// The destination is a mesh node - port typically 5222, or 22 for 'regular' SSH serves.
//	//
//	// After handshake, an initial message is sent, including informations about the current node.
//	//
//	// The remote can be a trusted VPN, an untrusted AP/Gateway, a peer (link local or with public IP),
//	// or a child. The subsriptions are used to indicate what messages will be forwarded to the server.
//	// Typically VPN will receive all events, AP will receive subset of events related to topology while
//	// child/peer only receive directed messages.
//	DialMUX(addr string, pub []byte, subs []string) (MuxedConn, error)
//}

//// A Connection that can multiplex.
//// Will dial a stream, may also accept streams and dispatch them.
////
//// For example SSHClient, SSHServer, Quic can support this.
//type MuxedConn interface {
//	// DialProxy will use the remote gateway to jump to
//	// a different destination, indicated by stream.
//	// On return, the stream ServerOut and ServerIn will be
//	// populated, and connected to stream Dest.
//	// deprecated:  use CreateStream
//	DialProxy(tp *Stream) error
//
//	// The VIP of the remote host, after authentication.
//	RemoteVIP() net.IP
//
//	// Wait for the conn to finish.
//	Wait() error
//}
//


// Textual representation of the node registration data.
func (n *DMNode) String() string {
	b, _ := json.Marshal(n)
	return string(b)
}

// Return the list of gateways for the node, starting with the link local if any.
func (n *DMNode) GWs() []*net.UDPAddr {
	res := []*net.UDPAddr{}

	if n.Last4 != nil {
		res = append(res, n.Last4)
	}
	if n.Last6 != nil {
		res = append(res, n.Last6)
	}
	return res
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
