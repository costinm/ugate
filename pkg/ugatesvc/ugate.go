package ugatesvc

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/auth"
	"github.com/costinm/ugate/pkg/cfgfs"
	msgs "github.com/costinm/ugate/webpush"
)

type StartFunc func(ug *UGate)
var InitHooks []func(gate *UGate) StartFunc


type UGate struct {

	Config *ugate.GateCfg

	// Default dialer used to connect to host:port extracted from metadata.
	// Defaults to net.Dialer, making real connections.
	//
	// Can be replaced with a mux or egress dialer or router for
	// integration.
	parentDialer ugate.ContextDialer


	DefaultRoute *ugate.Route

	// Handlers for incoming connections - local accept or forwarding.
	Mux *http.ServeMux

	// template, used for TLS connections and the host ID
	TLSConfig *tls.Config

	H2Handler *H2Transport

	// Direct Nodes by interface address (which is derived from public key). This includes only
	// directly connected notes - either Wifi on same segment, or VPNs and
	// connected devices - with TLS termination and mutual auth. The nodes typically
	// have a multiplexed connection.
	Nodes     map[uint64]*ugate.DMNode
	NodesByID map[string]*ugate.DMNode

	// Active connection by internal stream ID.
	// Tracks incoming Streams - if the stream is getting proxied to
	// a net.Conn or Conn, the dest will not be tracked here.
	ActiveTcp map[int]*ugate.Conn

	m    sync.RWMutex

	Auth *auth.Auth

	Msg *msgs.Mux

	MuxDialers    map[string]ugate.MuxDialer
	DNS           ugate.UDPHandler
	UDPHandler    ugate.UDPHandler
}

func NewConf(base ...string) ugate.ConfStore {
	return cfgfs.NewConf(base...)
}
func Get(h2 ugate.ConfStore, name string, to interface{}) error {
	raw, err := h2.Get(name)
	if err != nil {
		log.Println("name:", err)
		raw = []byte("{}")
		//return nil
	}
	if len(raw) > 0 {
		// TODO: detect yaml or proto ?
		if err := json.Unmarshal(raw, to); err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

func NewGate(d ugate.ContextDialer, a *auth.Auth, cfg *ugate.GateCfg, cs ugate.ConfStore) *UGate {
	ug := New(cs, a, cfg)
	if d != nil {
		ug.parentDialer = d
	}
	go ug.H2Handler.UpdateReverseAccept()
	ug.DefaultPorts(ug.Config.BasePort)

	ug.Start()
	return ug
}

func New(cs ugate.ConfStore, a *auth.Auth, cfg *ugate.GateCfg) *UGate {
	if cs == nil {
		cs = cfgfs.NewConf()
	}
	if cfg == nil {
		bp := ugate.ConfInt(cs, "BASE_PORT", 15000)

		cfg = &ugate.GateCfg{
			BasePort: bp,
		}
	}
	if cfg.H2R == nil {
		cfg.H2R = map[string]string{}
	}
	if cfg.Hosts == nil {
		cfg.Hosts = map[string]*ugate.DMNode{}
	}
	if cfg.Listeners == nil {
		cfg.Listeners = map[string]*ugate.Listener{}
	}
	if cfg.Routes == nil {
		cfg.Routes = map[string]*ugate.Route{}
	}

	// Merge 'ugate' JSON config, from config store.
	Get(cs, "ugate", cfg)

	if cfg.Domain == "" {
		cfg.Domain = ugate.ConfStr(cs, "DOMAIN", "c1.webinf.info")
		if len(cfg.H2R) == 0 {
			cfg.H2R[cfg.Domain] = ""
		}
	}

	if a == nil {
		a = auth.NewAuth(cs, cfg.Name, cfg.Domain)
	}

	ug := &UGate{
		parentDialer: &net.Dialer{},
		MuxDialers: map[string]ugate.MuxDialer{},
		Config:       cfg,
		NodesByID:    map[string]*ugate.DMNode{},
		Nodes:        map[uint64]*ugate.DMNode{},
		Mux:          http.NewServeMux(),
		Auth:         a,
		ActiveTcp:    map[int]*ugate.Conn{},
		DefaultRoute: &ugate.Route{
			Protocol: "OriginalDST",
		},
		Msg: msgs.DefaultMux,
	}

	if l, ok := cfg.Routes["*"]; ok {
		ug.DefaultRoute = l
	}


	ug.TLSConfig = &tls.Config{
		//MinVersion: tls.VersionTLS13,
		//PreferServerCipherSuites: ugate.preferServerCipherSuites(),
		InsecureSkipVerify: true,                  // This is not insecure here. We will verify the cert chain ourselves.
		ClientAuth:         tls.RequestClientCert, // not require - we'll fallback to JWT
		Certificates:       []tls.Certificate{*a.Cert}, // a.TlsCerts,
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
			return ug.Auth.Cert, nil
		},

		//SessionTicketsDisabled: true,
	}

	msgs.InitMux(ug.Msg,ug.Mux, ug.Auth)

	ug.H2Handler, _ = NewH2Transport(ug)

	ug.Mux.Handle("/debug/", http.DefaultServeMux)
	ug.Mux.HandleFunc("/dmesh/tcpa", ug.HttpTCP)
	ug.Mux.HandleFunc("/dmesh/rd", ug.HttpNodesFilter)
	ug.Mux.Handle("/debug/echo/", &EchoHandler{})
	ug.Mux.HandleFunc("/.well-known/openid-configuration", ug.Auth.HandleDisc)
	ug.Mux.HandleFunc("/jwks", ug.Auth.HandleJWK)
	ug.Mux.HandleFunc("/sts", ug.Auth.HandleSTS)

	return ug
}

// Start listening on all configured ports.
func (ug *UGate) Start() {
	// Explicit TCP forwarders.
	for k, t := range ug.Config.Listeners {
		if strings.HasPrefix(k, "udp://") {
			continue
		}
		t.Address = k
		ug.StartListener(t)
	}

	log.Println("Starting uGate ", ug.Config.Name,
		ug.Config.BasePort,
		auth.IDFromPublicKey(auth.PublicKey(ug.Auth.Cert.PrivateKey)),
		ug.Auth.VIP6)
}


// Expects the result to be validated and do ALPN.
//func (ug *UGate) DialTLS(net, addr string, tc *tls.Config) {
//
//}

func NewDMNode() *ugate.DMNode {
	now := time.Now()
	return &ugate.DMNode{
		Labels:       map[string]string{},
		FirstSeen:    now,
		LastSeen:     now,
		NodeAnnounce: &ugate.NodeAnnounce{},
	}
}

// GetNode returns a node, using an encoded id string.
//
//
func (ug *UGate) GetNode(id string) *ugate.DMNode {
	ug.m.RLock()
	n := ug.NodesByID[id]
	if n == nil {
		n = ug.Config.Hosts[id]
	}
	// Make sure it is set correctly.
	if n != nil && n.ID == "" {
		n.ID = id
	}
	ug.m.RUnlock()
	return n
}

// GetOrAddNode will get a node, and if not found create one,
// updating "FirstSeen". LastSeen will be update as well.
// NodesByID will be updated.
//
// id is a hostname or meshid, without port.
func (ug *UGate) GetOrAddNode(id string) *ugate.DMNode {
	ug.m.Lock()
	n := ug.NodesByID[id]
	if n == nil {
		n = ug.Config.Hosts[id]
	}
	// Make sure it is set correctly.
	if n != nil && n.ID == "" {
		n.ID = id
	}
	if n == nil {
		n = &ugate.DMNode{
			FirstSeen: time.Now(),
			ID:        id,
		}
		ug.NodesByID[id] = n
	}
	n.LastSeen = time.Now()
	ug.m.Unlock()
	return n
}

// All streams must call this method, and defer OnStreamDone
func (ug *UGate) OnStream(s *ugate.Conn) {
	ug.m.Lock()
	ug.ActiveTcp[s.StreamId] = s
	ug.m.Unlock()

	ugate.TcpConActive.Add(1)
	ugate.TcpConTotal.Add(1)
}

// Called at the end of the connection handling. After this point
// nothing should use or refer to the connection, both proxy directions
// should already be closed for write or fully closed.
func (ug *UGate) OnStreamDone(str *ugate.Conn) {

	ug.m.Lock()
	delete(ug.ActiveTcp, str.StreamId)
	ug.m.Unlock()
	ugate.TcpConActive.Add(-1)
	// TODO: track multiplexed streams separately.
	if str.ReadErr != nil {
		ugate.VarzSErrRead.Add(1)
	}
	if str.WriteErr != nil {
		ugate.VarzSErrWrite.Add(1)
	}
	if str.ProxyReadErr != nil {
		ugate.VarzCErrRead.Add(1)
	}
	if str.ProxyWriteErr != nil {
		ugate.VarzCErrWrite.Add(1)
	}

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

	if ugate.NoEOF(str.ReadErr) != nil || str.WriteErr != nil {
		log.Println(str.StreamId, "AE:", "Err in:", str.ReadErr, str.WriteErr)
	}
	if ugate.NoEOF(str.ProxyReadErr) != nil || str.ProxyWriteErr != nil {
		log.Println(str.StreamId, "AE:", "Err out:", str.ProxyReadErr, str.ProxyWriteErr)
	}
	if !str.Closed {
		str.Close()
	}

	ug.OnSClose(str, str.RemoteAddr())

}

// RemoteID returns the node ID based on authentication.
//
func RemoteID(s *ugate.Conn)  string {
	if s.TLS == nil {
		return ""
	}
	if len(s.TLS.PeerCertificates) == 0 {
		return ""
	}
	pk, err := auth.PubKeyFromCertChain(s.TLS.PeerCertificates)
	if err != nil {
		return ""
	}

	return auth.IDFromPublicKey(pk)
}

func (ug *UGate) Close() error {
	var err error
	for _, p := range ug.Config.Listeners {
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

// Setup default ports, using base port.
// For Istio, should be 15000. If running in Knative, use PORT and start
// only a H2 listener.
//
// Will run:
// - plaintext HTTP/1 or H2 - on PORT or base (15000)
// - BTS on 15007 (or 443 if running as root)
func (ug *UGate) DefaultPorts(base int) error {
	// Set if running in a knative env, or if an Envoy runs as a sidecar to handle
	// TLS, QUIC, H2. In this mode only standard H2/MASQUE are supported, with
	// reverse connections over POST or websocket.
	knativePort := os.Getenv("PORT")
	haddr := ""
	if knativePort != "" {
		haddr = ":" + knativePort
	} else {
		haddr = fmt.Sprintf("0.0.0.0:%d", base)
	}
	// ProtoHTTP detects H1/H2 and sends to ug.H2Handler
	// That deals with auth and dispatches to ugate.Mux
	ug.StartListener(&ugate.Listener{
		Address: haddr,
		Protocol: ugate.ProtoHTTP,
	})

	// KNative doesn't support other ports by default - but still register them
	btsAddr := fmt.Sprintf("0.0.0.0:%d", base+ugate.PORT_BTS)
	btscAddr := fmt.Sprintf("0.0.0.0:%d", base+ugate.PORT_BTSC)

	// Main BTS port, with TLS certificates
	// Normally should be 443 for SNI gateways, when running as root
	// Use iptables to redirect, or an explicit config for port 443 if running as root.
	ug.StartListener(&ugate.Listener{
		Address:  btsAddr,
		Protocol: ugate.ProtoBTS,
		ALPN: []string{"h2","h2r"},
	})
	ug.StartListener(&ugate.Listener{
		Address:  btscAddr,
		Protocol: ugate.ProtoBTSC,
	})
	if os.Getuid() == 0 {
		ug.StartListener(&ugate.Listener{
			Address:  "0.0.0.0:443",
			Protocol: ugate.ProtoTLS,
			ALPN: []string{"h2","h2r"},
		})
		ug.StartListener(&ugate.Listener{
			Address:  "0.0.0.0:80",
			Protocol: ugate.ProtoHTTP,
		})
	}

	return nil
}

// Based on the port in the Dest, find the Listener config.
// Used when the dest IP:port is extracted from the metadata
func (ug *UGate) FindRouteIn(m *ugate.Conn) *ugate.Route {
	//_, p, _ := net.SplitHostPort(m.Dest)
	//l := ug.Config.Listeners[":"+p]
	//if l != nil {
	//	return l
	//}

	//l := ug.Config.Listeners["-:"+p]
	//if l != nil {
	//	return &l.Route
	//}
	return ug.DefaultRoute
}

// FindRouteOut will use the IP in Dest, and find the cluster
// and endpoints.
func (ug *UGate) FindRouteOut(m *ugate.Conn) *ugate.Route {
	l := ug.Config.Routes[m.Dest]
	if l != nil {
		return l
	}

	h, p, _ := net.SplitHostPort(m.Dest)
	l = ug.Config.Routes[h]
	if l != nil {
		return l
	}
	l = ug.Config.Routes[":"+p]
	if l != nil {
		return l
	}
	return ug.DefaultRoute
}

func (ug *UGate) FindRoutePrefix(dstaddr net.IP, p uint16, prefix string) *ugate.Route {
	port := ":" + strconv.Itoa(int(p))
	l := ug.Config.Routes[prefix + dstaddr.String() + port]
	if l != nil {
		return l
	}

	l = ug.Config.Routes[prefix + port]
	if l != nil {
		return l
	}

	l = ug.Config.Routes[prefix + "-" + port]
	if l != nil {
		return l
	}
	return ug.DefaultRoute
}


// HandleStream is called for accepted (incoming) streams.
//
// Multiplexed streams ( H2, SNI ) also call this method.
//
// At this point the stream has the metadata:
//
// - Listener - actual port that accepted connection.
// - Dest - SNI, Host, original dest for iptables, listener's forward addr
// - RequestURI
// - Host
// - Headers
// - TLS context
//
// In addition TrackStreamIn has been called.
// This is a blocking method.
func (ug *UGate) HandleStream(str *ugate.Conn) error {
	if str.Route == nil {
		str.Route = ug.FindRouteOut(str)
	}
	route := str.Route

	//if route.Protocol == ugate.ProtoBTS || route.Protocol == ugate.ProtoBTSC {
	//	panic("Should not happen")
	//	str.PostDial(str, nil)
	//	// TLS is already wrapped for BTS
	//	return ug.H2Handler.HandleHTTPS(str)
	//}

	if route.ForwardTo != "" {
		str.Dest = route.ForwardTo
	}

	if route.Handler == nil && strings.HasPrefix(route.ForwardTo, "/") {
		// TODO: register handlers
		if route.ForwardTo == "/echo" {
			route.Handler = &EchoHandler{}
		}
	}

	// Config has an in-process handler - not forwarding (or the handler may
	// forward).
	if route.Handler != nil {
		// SOCKS and others need to send something back - we don't
		// have a real connection, faking it.
		str.PostDial(str, nil)
		str.Dest = fmt.Sprintf("%v", route.Handler)
		err:= route.Handler.Handle(str)
		str.Close()
		return err
	}

	// By default, dial out
	return ug.DialAndProxy(str)
}



func (gw *UGate) OnMuxClose(dm *ugate.DMNode) {
	if _, f := gw.Config.H2R[dm.ID]; !f {
		return
	}
	gw.H2Handler.maintainPinnedConnection(dm, nil)

}

var LogClose = true

// OnHClose called on http close
func (gw *UGate) OnHClose(s string, id string, san string, r *http.Request, since time.Duration) {
	if !gw.Config.NoAccessLog {
		log.Println("HTTP", r.Method, r.URL, r.Proto, r.Header, id, san, r.RemoteAddr, since)
	}
}

func (gw *UGate) OnSClose(str *ugate.Conn, addr net.Addr) {
	if !gw.Config.NoAccessLog {
		if str.ReadErr != nil || str.WriteErr != nil {
			log.Printf("%d AC: src=%s://%v dst=%s rcv=%d/%d snd=%d/%d la=%v ra=%v op=%v %v %v",
				str.StreamId,
				str.Type, addr,
				str.Dest,
				str.RcvdPackets, str.RcvdBytes,
				str.SentPackets, str.SentBytes,
				time.Since(str.LastWrite),
				time.Since(str.LastRead),
				int64(time.Since(str.Open).Seconds()),
				str.ReadErr, str.WriteErr)
			return
		}
		log.Printf("AC: %d src=%s://%v dst=%s rcv=%d/%d snd=%d/%d la=%v ra=%v op=%v",
			str.StreamId,
			str.Type, addr,
			str.Dest,
			str.RcvdPackets, str.RcvdBytes,
			str.SentPackets, str.SentBytes,
			time.Since(str.LastWrite),
			time.Since(str.LastRead),
			int64(time.Since(str.Open).Seconds()))

	}
}
