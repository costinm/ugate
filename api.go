package ugate

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
)

// Implemented by upgate

type CloseWriter interface {
	CloseWrite() error
}

type CloseReader interface {
	CloseRead() error
}


const ProtoTLS = "tls"

// autodetected for TLS.
const ProtoH2 = "h2"
const ProtoHTTP = "http" // 1.1

const ProtoHAProxy = "haproxy"

// Egress capture - for dedicated listeners
const ProtoSocks = "socks5"
const ProtoIPTables = "iptables"
const ProtoIPTablesIn = "iptables-in"
const ProtoConnect = "connect"

type MetaConn interface {
	net.Conn
	//http.ResponseWriter

	// Returns request metadata. This is a subset of
	// http.Request. An adapter exists to convert this into
	// a proper http.Request and use normal http.Handler and
	// RoundTrip.
	Meta() *Stream
}

// Connection that supports proxying. If a connection doesn't support
// this, it can be wrapped in a RawConn.
type ProxyConn interface {
	// Will be called after the handler has dialed.
	// For internally handled connections, will also be called with a local
	// conn. Used to return status for socks5, connect, etc.
	PostDial(net.Conn, error)
	ProxyTo(net.Conn) error
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
	// If Host is specified, it is used instead of port.
	Port int `json:"port,omitempty"`

	// Host address (ex :8080). This is the requested address - if busy :0 will be used instead, and Port
	// will be the actual port
	// TODO: UDS
	// TODO: indicate TLS SNI binding.
	Host string

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


type Node struct {
	// VIP is the mesh specific IP6 address. The 'network' identifies the master node, the
	// link part is the sha of the public key. This is a byte[16].
	// Last 8 bytes as uint64 are the primary key in the map.
	VIP net.IP `json:"vip,omitempty"`

	// IPFS CID: sha256 (32B) or ED pub key (32B),
	// Doesn't fit as HEX in DNS 63 - b32 can be used instead, 52 chars

	// Pub
	PublicKey []byte `json:"pub,omitempty"`

	// Multiplex connection accepted or dialed from the node
	// Incoming connections are dispatched on the gateway. This allows
	// opening streams to or via the node.
	mux ContextDialer

	Labels map[string]string `json:"l,omitempty"`


}

type Host struct {
	// Address and port of a HTTP server to forward the domain.
	Addr string

	// Directory to serve static files. Used if Addr not set.
	Dir string
	Mux http.Handler `json:"-"`
}

// Configuration for the Gateway.
//
type GateCfg struct {
	BasePort int `json:"basePort,omitempty"`

	// Port proxies: will register a listener for each port, forwarding to the
	// given address.
	Listeners []*ListenerConf `json:"TcpProxy,omitempty"`

	// Set of hosts with certs to configure in the h2 server.
	// The cert is expected in CertDir/HOSTNAME.[key,crt]
	// The server will terminate TLS and HTTP, forward to the host as plain text.
	Hosts map[string]*Host `json:"Hosts,omitempty"`

	// Proxy requests to hosts (external or mesh) using the VIP of another node.
	Via map[string]string `json:"Via,omitempty"`

	// VIP of the default egress node, if no 'via' is set.
	Egress string
}

