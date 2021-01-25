package ugate

import (
	"context"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
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
	PORT_SNI = 3
	PORT_HTTPS = 7
)

// Node information, based on registration info or discovery.
type DMNode struct {
	// VIP is the mesh specific IP6 address. The 'network' identifies the master node, the
	// link part is the sha of the public key. This is a byte[16].
	VIP net.IP `json:"vip,omitempty"`

	// Public key. If empty, the VIP should be set.
	PublicKey []byte `json:"pub,omitempty"`

	// Information from the node - from an announce or message.
	NodeAnnounce *NodeAnnounce `json:"info,omitempty"`

	// Primary/main address and port of the BTS endpoint.
	// Can be a DNS name that resolves, or some other node.
	// Individual IPs (relay, etc) will be in the info.addrs field
	Addr string `json:"addr,omitempty"`

	Labels map[string]string `json:"l,omitempty"`

	Bacokff time.Duration `json:"-"`

	// Last LL GW address used by the peer.
	// Public IP addresses are stored in Reg.IPs.
	// If set, returned as the first address in GWs, which is used to connect.
	// This is not sent in the registration - but extracted from the request
	// remote address.
	GW *net.UDPAddr `json:"gw,omitempty"`

	// Set if the gateway has an active incoming connection from this
	// node, with the node acting as client.
	// Streams will be forwarded to the node using special 'accept' mode.
	// This is similar with PUSH in H2.
	// Deprecated
	TunSrv MuxedConn `json:"-"`

	// Existing tun to the remote node, previously dialed.
	// Deprecated
	TunClient MuxedConn `json:"-"`

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

	h2r *http2.ClientConn `json:"-"`

	Mux http.Handler `json:"-"`
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
	Listener net.Listener `json:-`

	// Must block until the connection is fully handled !
	Handler ConHandler `json:-`

	gate *UGate `json:-`
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
// packets back to the virtual interface
type UdpWriter interface {
	WriteTo(data []byte, dstAddr *net.UDPAddr, srcAddr *net.UDPAddr) (int, error)
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



// JSON representation of the kube config
type KubeConfig struct {
	// Must be v1
	ApiVersion string `json:"apiVersion"`
	// Must be Config
	Kind string `json:"kind"`

	// Clusters is a map of referencable names to cluster configs
	Clusters []KubeNamedCluster `json:"clusters"`

	// AuthInfos is a map of referencable names to user configs
	Users []KubeNamedUser `json:"users"`

	// Contexts is a map of referencable names to context configs
	Contexts []KubeNamedContext `json:"contexts"`

	// CurrentContext is the name of the context that you would like to use by default
	CurrentContext string `json:"current-context"`
}

type KubeNamedCluster struct {
	Name string `json:"name"`
	Cluster KubeCluster `json:"cluster"`
}
type KubeNamedUser struct {
	Name string `json:"name"`
	User AuthInfo `json:"user"`
}
type KubeNamedContext struct {
	Name string `json:"name"`
	Context Context `json:"context"`
}

type KubeCluster struct {
	// LocationOfOrigin indicates where this object came from.  It is used for round tripping config post-merge, but never serialized.
	// +k8s:conversion-gen=false
	//LocationOfOrigin string
	// Server is the address of the kubernetes cluster (https://hostname:port).
	Server string `json:"server"`
	// InsecureSkipTLSVerify skips the validity check for the server's certificate. This will make your HTTPS connections insecure.
	// +optional
	InsecureSkipTLSVerify bool `json:"insecure-skip-tls-verify,omitempty"`
	// CertificateAuthority is the path to a cert file for the certificate authority.
	// +optional
	CertificateAuthority string `json:"certificate-authority,omitempty"`
	// CertificateAuthorityData contains PEM-encoded certificate authority certificates. Overrides CertificateAuthority
	// +optional
	CertificateAuthorityData []byte `json:"certificate-authority-data,omitempty"`
	// Extensions holds additional information. This is useful for extenders so that reads and writes don't clobber unknown fields
	// +optional
	//Extensions map[string]runtime.Object `json:"extensions,omitempty"`
}

// AuthInfo contains information that describes identity information.  This is use to tell the kubernetes cluster who you are.
type AuthInfo struct {
	// LocationOfOrigin indicates where this object came from.  It is used for round tripping config post-merge, but never serialized.
	// +k8s:conversion-gen=false
	//LocationOfOrigin string
	// ClientCertificate is the path to a client cert file for TLS.
	// +optional
	ClientCertificate string `json:"client-certificate,omitempty"`
	// ClientCertificateData contains PEM-encoded data from a client cert file for TLS. Overrides ClientCertificate
	// +optional
	ClientCertificateData []byte `json:"client-certificate-data,omitempty"`
	// ClientKey is the path to a client key file for TLS.
	// +optional
	ClientKey string `json:"client-key,omitempty"`
	// ClientKeyData contains PEM-encoded data from a client key file for TLS. Overrides ClientKey
	// +optional
	ClientKeyData []byte `json:"client-key-data,omitempty"`
	// Token is the bearer token for authentication to the kubernetes cluster.
	// +optional
	Token string `json:"token,omitempty"`
	// TokenFile is a pointer to a file that contains a bearer token (as described above).  If both Token and TokenFile are present, Token takes precedence.
	// +optional
	TokenFile string `json:"tokenFile,omitempty"`
	// Impersonate is the username to act-as.
	// +optional
	//Impersonate string `json:"act-as,omitempty"`
	// ImpersonateGroups is the groups to imperonate.
	// +optional
	//ImpersonateGroups []string `json:"act-as-groups,omitempty"`
	// ImpersonateUserExtra contains additional information for impersonated user.
	// +optional
	//ImpersonateUserExtra map[string][]string `json:"act-as-user-extra,omitempty"`
	// Username is the username for basic authentication to the kubernetes cluster.
	// +optional
	Username string `json:"username,omitempty"`
	// Password is the password for basic authentication to the kubernetes cluster.
	// +optional
	Password string `json:"password,omitempty"`
	// AuthProvider specifies a custom authentication plugin for the kubernetes cluster.
	// +optional
	//AuthProvider *AuthProviderConfig `json:"auth-provider,omitempty"`
	// Exec specifies a custom exec-based authentication plugin for the kubernetes cluster.
	// +optional
	//Exec *ExecConfig `json:"exec,omitempty"`
	// Extensions holds additional information. This is useful for extenders so that reads and writes don't clobber unknown fields
	// +optional
	//Extensions map[string]runtime.Object `json:"extensions,omitempty"`
}

// Context is a tuple of references to a cluster (how do I communicate with a kubernetes cluster), a user (how do I identify myself), and a namespace (what subset of resources do I want to work with)
type Context struct {
	// Cluster is the name of the cluster for this context
	Cluster string `json:"cluster"`
	// AuthInfo is the name of the authInfo for this context
	AuthInfo string `json:"user"`
	// Namespace is the default namespace to use on unspecified requests
	// +optional
	Namespace string `json:"namespace,omitempty"`
}
