package ugate

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"expvar"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	streamIds = uint32(0)
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
	tcpConTotal = expvar.NewInt("gate:tcp:total")

	// Managed by updateStatsOnClose - including error cases.
	tcpConActive = expvar.NewInt("gate:tcp:active")
)

type UGate struct {
	// Actual (real) port listeners, key is the host:port
	// Typically host is 0.0.0.0 or 127.0.0.1 - may also be one of the
	// local addresses.
	Listeners map[string]*Listener

	Config *GateCfg

	// Default dialer used to connect to host:port extracted from metadata.
	// Defaults to net.Dialer, making real connections.
	// Can be replaced with a mux or egress dialer or router.
	Dialer ContextDialer

	// Configurations, keyed by host + port and URL.
	Conf map[string]*Listener

	DefaultListener *Listener

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
	Nodes     map[uint64]*DMNode
	NodesByID map[string]*DMNode

	// Active connection by internal stream ID.
	// Tracks incoming Streams - if the stream is getting proxied to
	// a net.Conn or Stream, the dest will not be tracked here.
	ActiveTcp map[int]*Stream

	m     sync.RWMutex
	Auth  *Auth
}

func NewGate(d ContextDialer, auth *Auth, cfg *GateCfg) *UGate {
	if auth == nil {
		auth = NewAuth(nil, "", "cluster.local")
	}
	if cfg == nil {
		cfg = &GateCfg{

		}
	}

	ug := &UGate{
		Listeners: map[string]*Listener{},
		Dialer:    d,
		Config:    cfg,
		NodesByID: map[string]*DMNode{},
		Nodes:     map[uint64]*DMNode{},
		Conf:      map[string]*Listener{},
		Mux:       http.NewServeMux(),
		EgressMux: http.NewServeMux(),
		Auth:      auth,
		ActiveTcp: map[int]*Stream{},
		DefaultListener: &Listener{
		},
	}


	ug.TLSConfig = &tls.Config{
		MinVersion: tls.VersionTLS13,
		//PreferServerCipherSuites: ugate.preferServerCipherSuites(),
		InsecureSkipVerify: true,                  // This is not insecure here. We will verify the cert chain ourselves.
		ClientAuth:         tls.RequestClientCert, // not require - we'll fallback to JWT
		Certificates:       auth.tlsCerts,
		VerifyPeerCertificate: func(_ [][]byte, _ [][]*x509.Certificate) error {
			panic("tls config not specialized for peer")
		},
		NextProtos: []string{"istio", "h2"},
		// Will only be called if client supplies SNI and Certificates empty
		GetCertificate: func(ch *tls.ClientHelloInfo) (*tls.Certificate, error) {
			// Log on each new TCP connection, after client hello
			//
			log.Printf("Server/NewConn/CH %s %v %v", ch.ServerName, ch.SupportedProtos, ch.Conn.RemoteAddr())
			// doesn't include :5228
			c, ok := auth.certMap[ch.ServerName]
			if ok {
				return c, nil
			}
			return &ug.Auth.tlsCerts[0], nil
		},

		//SessionTicketsDisabled: true,
	}

	ug.h2Handler, _ = NewH2Transport(ug)

	go ug.h2Handler.UpdateReverseAccept()

	ug.Mux.Handle("/debug/", http.DefaultServeMux)
	ug.Mux.HandleFunc("/dmesh/tcpa", ug.HttpTCP)
	ug.Mux.HandleFunc("/dmesh/rd", ug.HttpNodesFilter)
	ug.Mux.Handle("/debug/echo/", &EchoHandler{})

	ug.DefaultPorts(ug.Config.BasePort)

	// Explicit TCP forwarders.
	for k, t := range ug.Config.Listeners {
		t.Address = k
		ug.Add(t)
	}

	return ug
}

// Expects the result to be validated and do ALPN.
//func (ug *UGate) DialTLS(net, addr string, tc *tls.Config) {
//
//}

// New TLS or TCP connection.
func (ug *UGate) DialContext(ctx context.Context, netw, addr string) (net.Conn, error) {
	tcpC, err := ug.Dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	if "tls" == netw {
		// TODO: parse addr as URL or VIP6 extract peer ID
		return ug.NewTLSConnOut(ctx, tcpC, ug.TLSConfig, "", nil)
	}
	if "h2r" == netw {
		// TODO: parse addr as URL or VIP6 extract peer ID
		return ug.NewTLSConnOut(ctx, tcpC, ug.TLSConfig, "", []string{"h2r", "h2"})
	}

	return tcpC, err
}

func (ug *UGate) DialTLS(addr string, alpn []string) (*Stream, error) {
	ctx, cf := context.WithTimeout(context.Background(), 5*time.Second)
	tcpC, err := ug.Dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	// TODO: parse addr as URL or VIP6 extract peer ID
	t, err := ug.NewTLSConnOut(ctx, tcpC, ug.TLSConfig, "", alpn)
	if err != nil {
		return nil, err
	}
	cf()
	return t.Meta(), nil
}

func (ug *UGate) GetNode(id string) *DMNode {
	ug.m.RLock()
	n := ug.NodesByID[id]
	if n == nil {
		n = ug.Config.Hosts[id]
	}
	ug.m.RUnlock()
	return n
}

func (ug *UGate) GetOrAddNode(id string) *DMNode {
	ug.m.Lock()
	n := ug.NodesByID[id]
	if n == nil {
		n = &DMNode{
			FirstSeen: time.Now(),
		}
		ug.NodesByID[id] = n
	}
	n.LastSeen = time.Now()
	ug.m.Unlock()
	return n
}

// Add a real port listener on a port.
// Virtual listeners can be added to ug.Conf or the mux.
func (ug *UGate) Add(cfg *Listener) (*Listener, net.Addr, error) {
	cfg.gate = ug
	err := cfg.start()
	if err != nil {
		return nil, nil, err
	}
	ug.Listeners[cfg.Address] = cfg
	return cfg, cfg.Addr(), nil
}

func (ug *UGate) Close() error {
	var err error
	for _, p := range ug.Listeners {
		e := p.Close()
		if e != nil {
			err = err
		}
		delete(ug.Listeners, p.Address)
	}
	return err
}

// Setup default ports, using base port.
// For Istio, should be 15000
func (ug *UGate) DefaultPorts(base int) error {
	// Set if running in a knative env.
	knativePort := os.Getenv("PORT")
	if knativePort != "" {
		ug.Add(&Listener{
			Address: ":" + knativePort,
			Protocol: ProtoHTTP,
		})
		// KNative doesn't support other ports by default - but still register them
	}

	// Egress: iptables and SOCKS5
	// Not on localhost - redirect changes the port, keeps IP
	ug.Add(&Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", base+PORT_IPTABLES),
		Protocol: ProtoIPTables,
	})
	ug.Add(&Listener{
		Address:  fmt.Sprintf("127.0.0.1:%d", base+PORT_SOCKS),
		Protocol: ProtoSocks,
	})
	// TODO: add HTTP CONNECT for egress.

	// Ingress: iptables ( capture all incoming )
	ug.Add(&Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", base+PORT_IPTABLES_IN),
		Protocol: ProtoIPTablesIn,
	})
	// BTS - incoming, SNI, relay
	// Normally should be 443 for SNI gateways
	// Use iptables to redirect, or an explicit config for port 443 if running as root.
	// Based on SNI a virtual listener may be selected.
	ug.Add(&Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", base+PORT_SNI),
		Protocol: ProtoTLS,
	})

	ug.Add(&Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", base+PORT_HTTPS),
		Protocol: ProtoHTTPS,
		Handler:  ug.h2Handler,
	})
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", base), ug.Mux)
		if err != nil {
			log.Fatal(err)
		}
	}()

	return nil
}

func (ug *UGate) findCfgIptablesIn(bconn MetaConn) *Listener {
	m := bconn.Meta()
	_, p, _ := net.SplitHostPort(m.Dest)
	l := ug.Config.Listeners["-:"+p]
	if l != nil {
		return l
	}
	return ug.DefaultListener
}

func (ug *UGate) httpEgressProxy(pl *Listener, bconn *RawConn) error {
	return nil
}

