package ugate

import (
	"context"
	"io"
	"net"
	"net/http/httptrace"
	"time"
)

// Implemented by upgate

type CloseWriter interface {
	CloseWrite() error
}

// ---------- Structs

type Stats struct {
	//
	httptrace.ClientTrace

	Open time.Time
	LastRead time.Time
	LastWrite time.Time

	ReadBytes int
	ReadPackets int

	WriteBytes int
	WritePackets int

	StreamId int32

	// Type of the connection - empty is plain TCP, socks5, socks5IP, sni, ...
	Type string

	// Errors associated with this stream, read from or write to.
	ReadErr error
	WriteErr error
	ProxyReadErr error
	ProxyWriteErr error
}

// ListenerConf represents the configuration for an acceptor.
// Based on Istio/K8S Gateway models.
//
type ListenerConf struct {
	// Real port the listener is listening on, or 0 if the listener is not bound to a port (virtual, using mesh).
	Port int `json:"port,omitempty"`

	// Hostname selected by metadata (SNI, SOCKS, HTTP, etc)
	Hostname string `json:"hostname,omitempty"`

	// Port can have multiple protocols - will use auto-detection.
	// TLS (SNI matching), HTTP, HTTPS, SOCKS, etc
	// Fallback if no detection: TCP
	Protocol string

	// If empty, all families.
	// localhost, 0.0.0.0, ::, etc.
	BindIP string


	// Local address (ex :8080). This is the requested address - if busy :0 will be used instead, and Port
	// will be the actual port
	// TODO: UDS
	// TODO: indicate TLS SNI binding.
	Local string


	Name string

	// Remote where to forward the proxied connections
	// IP:port format, where IP can be a mesh VIP
	Remote string `json:"Remote,omitempty"`

	// TODO: ssh bind host/port as labels ?
	Endpoint ContextDialer `json:-`
}

//
type K8SListener struct {

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
	OnConn(reader *AcceptedConn) bool

	OnMeta(reader *AcceptedConn) bool

	OnConnClose(reader *AcceptedConn, err error) bool
}
