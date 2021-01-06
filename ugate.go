package ugate

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"expvar"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

var (
	streamIds = uint32(0)
)


var (
	VarzAccepted = expvar.NewInt("ugate.accepted")
	VarzAcceptErr = expvar.NewInt("ugate.acceptErr")

	// Proxy behavior
	VarzReadFrom = expvar.NewInt("ugate.sReadFrom")
	VarzReadFromC = expvar.NewInt("ugate.cReadFrom")

	VarzSErrRead = expvar.NewInt("ugate.sErrRead")
	VarzSErrWrite = expvar.NewInt("ugate.sErrWrite")
	VarzCErrRead = expvar.NewInt("ugate.cErrRead")
	VarzCErrWrite = expvar.NewInt("ugate.cErrWrite")

	VarzMaxRead = expvar.NewInt("ugate.maxRead")

	// Managed by 'NewTCPProxy' - before dial.
	tcpConTotal = expvar.NewInt("gate:tcp:total")

	// Managed by updateStatsOnClose - including error cases.
	tcpConActive = expvar.NewInt("gate:tcp:active")
)

func NewStream() *Stream {
	return &Stream{
		Request: &http.Request{
			Header:     make(http.Header),
			URL: 	&url.URL{},
		},
		Open: time.Now(),
	}
}
func NewStreamHeaders(h http.Header) *Stream {
	m := &Stream{
		Request: &http.Request{
			Header:     h,
			URL: 	&url.URL{},
		},
		Open: time.Now(),
	}
	m.Request.Host = h.Get(":authority")
	m.Request.URL, _ = url.Parse(h.Get(":path"))
	return m
}

func (ug *UGate) NewStreamRequest(r *http.Request, w http.ResponseWriter, con *Stream) *Stream {
	return &Stream{
		Request: r,
		Open: time.Now(),
		ServerIn: r.Body,
		ServerOut: w,
		ResponseHeader: w.Header(),
	}
}

func (s *Stream) Reset() {
	s.Open = time.Now()
	s.LastRead = time.Time{}
	s.LastWrite = time.Time{}

	s.RcvdBytes = 0
	s.SentBytes = 0
	s.RcvdPackets = 0
	s.SentPackets = 0

	s.ReadErr = nil
	s.WriteErr = nil
	s.Type = ""

	s.StreamId = int(atomic.AddUint32(&streamIds, 1))
}

type UGate struct {
	// Actual (real) port listeners, key is the host:port
	// Typically host is 0.0.0.0 or 127.0.0.1 - may also be one of the
	// local addresses.
	Listeners map[string]*PortListener

	Config *GateCfg

	// Default dialer used to connect to host:port extracted from metadata.
	// Defaults to net.Dialer, making real connections.
	// Can be replaced with a mux or egress dialer or router.
	Dialer ContextDialer

	// Configurations, keyed by host + port and URL.
	Conf            map[string]*ListenerConf
	DefaultListener *ListenerConf

	// Handlers for incoming connections - local accept or forwarding.
	Mux *http.ServeMux

	// Handlers for egress - source is local. Default is to forward using Dialer.
	EgressMux *http.ServeMux

	// Used to inject UDP packets into the host with custom source address.
	// Only set when a TUN or TPROXY are used.
	TUNUDPWriter UdpWriter

	// template, used for TLS connections and the host ID
	TLSConfig *tls.Config
	h2Handler *H2Transport

	// Direct Nodes by interface address (which is derived from public key). This includes only
	// directly connected notes - either Wifi on same segment, or VPNs and
	// connected devices - with TLS termination and mutual auth. The nodes typically
	// have a multiplexed connection.
	Nodes map[uint64]*DMNode

	// Active connection by internal stream ID.
	// Tracks incoming Streams - if the stream is getting proxied to
	// a net.Conn or Stream, the dest will not be tracked here.
	ActiveTcp map[int]*Stream

	m    sync.RWMutex
	Auth *Auth
}

func NewGate(d ContextDialer, auth *Auth) *UGate {
	if auth == nil {
		auth = NewAuth(nil, "", "cluster.local")
	}
	ug := &UGate{
		Listeners: map[string]*PortListener{},
		Dialer: d,
		Nodes: map[uint64]*DMNode{},
		Conf: map[string]*ListenerConf{},
		Mux: http.NewServeMux(),
		EgressMux: http.NewServeMux(),
		Auth: auth,
		ActiveTcp: map[int]*Stream{},
		DefaultListener: &ListenerConf{
		},
	}

	ug.TLSConfig = &tls.Config{
		MinVersion:               tls.VersionTLS13,
		//PreferServerCipherSuites: ugate.preferServerCipherSuites(),
		InsecureSkipVerify:       true, // This is not insecure here. We will verify the cert chain ourselves.
		ClientAuth:               tls.RequestClientCert, // not require - we'll fallback to JWT
		Certificates:             auth.tlsCerts,
		VerifyPeerCertificate: func(_ [][]byte, _ [][]*x509.Certificate) error {
			panic("tls config not specialized for peer")
		},
		NextProtos:             []string{"istio", "h2"},
		//SessionTicketsDisabled: true,
	}

	ug.h2Handler, _ = NewH2Transport(ug)

	return ug
}

// Expects the result to be validated and do ALPN.
//func (ug *UGate) DialTLS(net, addr string, tc *tls.Config) {
//
//}

// New TLS or TCP connection.
func (ug *UGate) DialContext(ctx context.Context, netw, addr string) (net.Conn, error) {
	tcpC, err := ug.Dialer.DialContext(ctx, "tcp", addr)

	if "tls" == netw {
		// TODO: parse addr as URL or VIP6 extract peer ID
		if err != nil {
			return nil, err
		}
		return NewTLSConnOut(ctx, tcpC, ug.TLSConfig, "", nil)
	}
	if "h2r" == netw {
		// TODO: parse addr as URL or VIP6 extract peer ID
		if err != nil {
			return nil, err
		}
		return NewTLSConnOut(ctx, tcpC, ug.TLSConfig, "", []string{"h2r", "h2"})
	}

	return tcpC, err
}

// Add a real port listener on a port.
// Virtual listeners can be added to ug.Conf or the mux.
func (ug *UGate) Add(cfg *ListenerConf) (*PortListener, net.Addr, error) {
	l, err := NewListener(ug, cfg)
	if err != nil {
		return nil, nil, err
	}
	ug.Listeners[l.cfg.Host] = l
	return l, l.Listener.Addr(), nil
}

func (ug *UGate) Close() error {
	var err error
	for _, p := range ug.Listeners {
		e := p.Close()
		if e != nil {
			err = err
		}
		delete(ug.Listeners, p.cfg.Host)
	}
	return err
}

// Setup default ports, using base port.
// For Istio, should be 15000
func (ug *UGate) DefaultPorts(base int) error {
	// Egress: iptables and SOCKS5
	// Not on localhost - redirect changes the port, keeps IP
	ug.Add(&ListenerConf{
		Port: base + 1,
		Protocol: ProtoIPTables,
	})
	ug.Add(&ListenerConf{
		Host:     fmt.Sprintf("127.0.0.1:%d", base + 9),
		Protocol: ProtoSocks,
	})
	// TODO: add HTTP CONNECT for egress.

	// Ingress: iptables ( capture all incoming )
	ug.Add(&ListenerConf{
		Port: base + 6,
		Protocol: ProtoIPTablesIn,
	})
	// BTS - incoming, SNI, relay
	// Normally should be 443 for SNI gateways
	// Use iptables to redirect, or an explicit config for port 443 if running as root.
	// Based on SNI a virtual listener may be selected.
	ug.Add(&ListenerConf{
		Port: base + 3,
		Protocol: ProtoTLS,
	})

	ug.Add(&ListenerConf{
		Port:      base + 7,
		Protocol:  ProtoTLS,
		TLSConfig: ug.TLSConfig,
		Handler:   ug.h2Handler,
	})
	return nil
}

func (ug *UGate) findCfg(bconn MetaConn) *ListenerConf {
	m := bconn.Meta()
	l := ug.Conf[m.Request.Host]
	if l != nil {
		return l
	}
	return ug.DefaultListener
}
