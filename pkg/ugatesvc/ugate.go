package ugatesvc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/auth"
	"github.com/costinm/ugate/pkg/msgs"
)


type UGate struct {
	// Actual (real) port listeners, key is the host:port
	// Typically host is 0.0.0.0 or 127.0.0.1 - may also be one of the
	// local addresses.
	Listeners map[string]*Listener

	Config *ugate.GateCfg

	// Default dialer used to connect to host:port extracted from metadata.
	// Defaults to net.Dialer, making real connections.
	// Can be replaced with a mux or egress dialer or router.
	Dialer ugate.ContextDialer

	// Configurations, keyed by host + port and URL.
	Conf map[string]*ugate.Listener

	DefaultListener *ugate.Listener

	// Handlers for incoming connections - local accept or forwarding.
	Mux *http.ServeMux

	// Handlers for egress - source is local. Default is to forward using Dialer.
	EgressMux *http.ServeMux

	// template, used for TLS connections and the host ID
	TLSConfig *tls.Config

	h2Handler *H2Transport

	// Direct Nodes by interface address (which is derived from public key). This includes only
	// directly connected notes - either Wifi on same segment, or VPNs and
	// connected devices - with TLS termination and mutual auth. The nodes typically
	// have a multiplexed connection.
	Nodes     map[uint64]*ugate.DMNode
	NodesByID map[string]*ugate.DMNode

	// Active connection by internal stream ID.
	// Tracks incoming Streams - if the stream is getting proxied to
	// a net.Conn or Stream, the dest will not be tracked here.
	ActiveTcp map[int]*ugate.Stream

	m     sync.RWMutex
	Auth  *auth.Auth

	Msg *msgs.Pubsub
}

func NewGate(d ugate.ContextDialer, a *auth.Auth, cfg *ugate.GateCfg, cs ugate.ConfStore) *UGate {
	if cfg == nil {
		// No config storage - test mode.
		if cs == nil {
			cfg = &ugate.GateCfg {
				BasePort: 0,
				Domain: "h.webinf.info",
			}
		} else {
			bp := ugate.ConfInt(cs, "BASE_PORT", 15000)

			cfg = &ugate.GateCfg{
				BasePort: bp,
				Domain:   ugate.ConfStr(cs, "DOMAIN", "h.webinf.info"),
				H2R: map[string]string{
					"c1.webinf.info": "",
				},
			}
		}
	}

	if cs != nil {
		Get(cs, "ugate", cfg)
	}

	if a == nil {
		a = auth.NewAuth(cs, cfg.Name, cfg.Domain)
	}

	ug := &UGate{
		Listeners: map[string]*Listener{},
		Dialer:    d,
		Config:    cfg,
		NodesByID: map[string]*ugate.DMNode{},
		Nodes:     map[uint64]*ugate.DMNode{},
		Conf:      map[string]*ugate.Listener{},
		Mux:       http.NewServeMux(),
		EgressMux: http.NewServeMux(),
		Auth:      a,
		ActiveTcp: map[int]*ugate.Stream{},
		DefaultListener: &ugate.Listener{
		},
		Msg: msgs.NewPubsub(),
	}


	ug.TLSConfig = &tls.Config{
		MinVersion: tls.VersionTLS13,
		//PreferServerCipherSuites: ugate.preferServerCipherSuites(),
		InsecureSkipVerify: true,                  // This is not insecure here. We will verify the cert chain ourselves.
		ClientAuth:         tls.RequestClientCert, // not require - we'll fallback to JWT
		Certificates:       a.TlsCerts,
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
			c, ok := a.CertMap[ch.ServerName]
			if ok {
				return c, nil
			}
			return &ug.Auth.TlsCerts[0], nil
		},

		//SessionTicketsDisabled: true,
	}

	ug.h2Handler, _ = NewH2Transport(ug)

	go ug.h2Handler.UpdateReverseAccept()

	ug.Mux.Handle("/debug/", http.DefaultServeMux)
	ug.Mux.HandleFunc("/dmesh/tcpa", ug.HttpTCP)
	ug.Mux.HandleFunc("/dmesh/rd", ug.HttpNodesFilter)
	ug.Mux.Handle("/debug/echo/", &EchoHandler{})
	ug.Mux.HandleFunc("/.well-known/openid-configuration", ug.Auth.HandleDisc)
	ug.Mux.HandleFunc("/jwks", ug.Auth.HandleJWK)
	ug.Mux.HandleFunc("/sts", ug.Auth.HandleSTS)

	ug.Mux.HandleFunc("/msg/", ug.Msg.HandleMsg)

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

// DialContext creates  connection to the remote addr.
// Supports:
// - tcp - normal tcp address, using the gate dialer.
// - tls - tls connection, using the gate workload identity.
// - h2r - h2r connection, suitable for reverse H2.
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

// DialTLS opens a direct TLS connection using the dialer for TCP.
// No peer verification - the returned stream will have the certs.
// addr is a real internet address, not a mesh one.
func (ug *UGate) DialTLS(addr string, alpn []string) (*ugate.Stream, error) {
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

func (ug *UGate) GetNode(id string) *ugate.DMNode {
	ug.m.RLock()
	n := ug.NodesByID[id]
	if n == nil {
		n = ug.Config.Hosts[id]
	}
	ug.m.RUnlock()
	return n
}

func (ug *UGate) GetOrAddNode(id string) *ugate.DMNode {
	ug.m.Lock()
	n := ug.NodesByID[id]
	if n == nil {
		n = &ugate.DMNode{
			FirstSeen: time.Now(),
		}
		ug.NodesByID[id] = n
	}
	n.LastSeen = time.Now()
	ug.m.Unlock()
	return n
}

func RemoteID(s *ugate.Stream)  string {
	if s.TLS == nil {
		return ""
	}
	pk, err := auth.PubKeyFromCertChain(s.TLS.PeerCertificates)
	if err != nil {
		return ""
	}

	return auth.IDFromPublicKey(pk)
}


// Add and start a real port listener on a port.
// Virtual listeners can be added to ug.Conf or the mux.
func (ug *UGate) Add(cfg *ugate.Listener) (*Listener, net.Addr, error) {
	ll := &Listener{Listener:*cfg}
	//ll.gate = ug
	err := ll.start(ug)
	if err != nil {
		return nil, nil, err
	}
	ug.Listeners[cfg.Address] = ll
	return ll, ll.Addr(), nil
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
		ug.Add(&ugate.Listener{
			Address: ":" + knativePort,
			Protocol: ugate.ProtoHTTP,
		})
		// KNative doesn't support other ports by default - but still register them
	}

	// Egress: iptables and SOCKS5
	// Not on localhost - redirect changes the port, keeps IP
	ug.Add(&ugate.Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", base+ugate.PORT_IPTABLES),
		Protocol: ugate.ProtoIPTables,
	})
	ug.Add(&ugate.Listener{
		Address:  fmt.Sprintf("127.0.0.1:%d", base+ugate.PORT_SOCKS),
		Protocol: ugate.ProtoSocks,
	})
	// TODO: add HTTP CONNECT for egress.

	// Ingress: iptables ( capture all incoming )
	ug.Add(&ugate.Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", base+ugate.PORT_IPTABLES_IN),
		Protocol: ugate.ProtoIPTablesIn,
	})
	// BTS - incoming, SNI, relay
	// Normally should be 443 for SNI gateways
	// Use iptables to redirect, or an explicit config for port 443 if running as root.
	// Based on SNI a virtual listener may be selected.
	ug.Add(&ugate.Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", base+ugate.PORT_SNI),
		Protocol: ugate.ProtoTLS,
	})

	ug.Add(&ugate.Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", base+ugate.PORT_HTTPS),
		Protocol: ugate.ProtoHTTPS,
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

func (ug *UGate) findCfgIptablesIn(bconn ugate.MetaConn) *ugate.Listener {
	m := bconn.Meta()
	_, p, _ := net.SplitHostPort(m.Dest)
	l := ug.Config.Listeners["-:"+p]
	if l != nil {
		return l
	}
	return ug.DefaultListener
}

func (ug *UGate) httpEgressProxy(pl *ugate.Listener, bconn *ugate.RawConn) error {
	return nil
}

