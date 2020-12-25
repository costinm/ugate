package ugate

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http/httptrace"
	"time"
)

// Implemented by upgate

type CloseWriter interface {
	CloseWrite() error
}

const ProtoTLS = "tls"
const ProtoH2 = "h2"
const ProtoHTTP = "http" // 1.1
const ProtoHAProxy = "haproxy"

// Egress capture - for dedicated listeners
const ProtoSocks = "socks5"
const ProtoIPTables = "iptables"
const ProtoIPTablesIn = "iptables-in"
const ProtoConnect = "connect"

// Connection with metadata and proxy support
type MetaConn interface {
	net.Conn
	Meta() *Stats

	// Will be called after the handler has dialed.
	// For internally handled connections, will also be called with a local
	// conn. Used to return status for socks5, connect, etc.
	PostDial(net.Conn, error)
	Proxy(net.Conn) error
}

type BufferedConn interface {
	net.Conn
	Buffer() []byte
	Fill()
	Proxy(net.Conn) error
}

// Stats and info about the request.
// Includes metadata.
type Stats struct {
	//
	httptrace.ClientTrace

	Open      time.Time
	LastRead  time.Time
	LastWrite time.Time

	ReadBytes   int
	ReadPackets int

	WriteBytes   int
	WritePackets int

	StreamId int32

	// Type of the connection - empty is plain TCP, socks5, socks5IP, sni, ...
	Type string

	// Destination of the connection, extracted from metadata or config.
	// Typically a DNS or IP address, can be a URL or path too.
	Target string

	// Errors associated with this stream, read from or write to.
	ReadErr       error
	WriteErr      error
	ProxyReadErr  error
	ProxyWriteErr error

	// If false, it's a dialed stream.
	Accepted bool

	Extra       map[interface{}]interface{}
	RemoteChain []*x509.Certificate

	// Context is associated with the connection at creation time.
	Context       context.Context
	ContextCancel context.CancelFunc
}

// ListenerConf represents the configuration for an acceptor on a port or addr:port
//
// For each port, metadata is optionally extracted - in particular a 'hostname', which
// is the destination IP or name:
// - socks5 dest
// - iptables original dst ( may be combined with DNS interception )
// - NAT dst address
// - SNI for TLS
// - :host header for HTTP
//
// Based on Istio/K8S Gateway models.
type ListenerConf struct {
	// Real port the listener is listening on.
	// If Local is specified, it is used instead of port.
	Port int `json:"port,omitempty"`

	// Local address (ex :8080). This is the requested address - if busy :0 will be used instead, and Port
	// will be the actual port
	// TODO: UDS
	// TODO: indicate TLS SNI binding.
	Local string

	// Extracted from all Gateways, based on same Port and address
	// WIP, not implemented
	Listeners map[string]*Listener
	// WIP, not implemented. All routes selected by the matcher in the listener for the hostname
	// * used for wildcard listener.
	TCPRoutes map[string]*TCPRoute

	// Port can have multiple protocols - will use auto-detection.
	// sni (SNI matching), HTTP, HTTPS, socks5, iptables, iptables_in, etc
	// Fallback if no detection: TCP
	Protocol string


	// Remote where to forward the proxied connections
	// IP:port format, where IP can be a mesh VIP
	Remote string `json:"Remote,omitempty"`

	// Per listener dialer. If nil the global one is used, which
	// defaults to net.Dialer.
	Dialer ContextDialer `json:-`

	// Custom listener - not guaranteed to return TCPConn
	Listener  net.Listener `json:-`
	// Must block until the connection is fully handled !
	Handler   ConHandler
	// Default config for the port.
	// SNI may override
	TLSConfig *tls.Config

	// Default outgoing TLS config for all requests on this port.
	RemoteTLS *tls.Config
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

// Interface implemented by Gateway.
// Implemented by net.Dialer, used in httpClient.
// Also implements x.net.proxy.ContextDialer - socks also implements it.
type ContextDialer2 interface {
	DialProxy(ctx context.Context,
			addr net.Addr, directClientAddr net.Addr,
			ctype string, meta ...string) (net.Conn, func(client net.Conn) error, error)
}

type ContextDialer interface {
	DialContext(ctx context.Context, net, addr string) (net.Conn, error)
}

// AcceptForwarder is used to tunnel accepted connections over a multiplexed stream.
// Implements -R in ssh.
// TODO: h2 implementation
// Used by acceptor.
type AcceptForwarder interface {
	// Called when a connection was accepted.
	//
	AcceptForward(in io.ReadCloser, out io.Writer,	remoteIP net.IP, remotePort int)
}

type ConnInterceptor interface {
	OnConn(reader MetaConn) bool

	OnMeta(reader MetaConn) bool

	OnConnClose(reader MetaConn, err error) bool
}

type ConHandler interface {
	Handle(conn MetaConn) error
}
