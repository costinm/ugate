package quic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/costinm/meshauth/pkg/certs"

	"github.com/costinm/ugate/nio"
	"github.com/quic-go/quic-go"

	"github.com/quic-go/webtransport-go"
)

// Quic is the adapter to QUIC/H3/MASQUE for uGate.
type Quic struct {
	Auth *certs.Cert

	// Incoming streams are mapped to HTTP
	//HTTPHandler http.Handler
	//server      *http3.Server

	tlsServerConfig *tls.Config

	Mux *http.ServeMux

	Dialer    ContextDialer
	Address   string

	MeshTrust *certs.MeshTrust

	Peers map[string]*Peer
}

type ContextDialer interface {
	DialContext(ctx context.Context, net, addr string) (net.Conn, error)
}


type Peer struct {
}

// DataSreamer is implemented by response writer on server side to take over the stream.
type DataStreamer interface {
	DataStream() quic.Stream
}


func New() *Quic {
	return &Quic{
		Address: ":4443",
	}
}

func (qa *Quic) Provision(ctx context.Context) error {
	os.Setenv("QUIC_GO_LOG_LEVEL", "DEBUG")

	if qa.Mux == nil {
		qa.Mux = http.NewServeMux()
	}

	if qa.Dialer == nil {
		qa.Dialer = &net.Dialer{}
	}

	wts := webtransport.Server{}
	qa.Mux.HandleFunc("/.well-known/webtransport", func(writer http.ResponseWriter, request *http.Request) {
		// expects CONNECT - can't be handled by muxConn
		wts.Upgrade(writer, request)
	})


	mtlsServerConfig := qa.Auth.GenerateTLSConfigServer(qa.MeshTrust)

	// Overrides
	mtlsServerConfig.NextProtos = []string{"h3r", "h3-34"}

	// called with ClientAuth is RequestClientCert or RequireAnyClientCert
	mtlsServerConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		certs := make([]*x509.Certificate, len(rawCerts))
		for i, certEntry := range rawCerts {
			certs[i], _ = x509.ParseCertificate(certEntry)
		}
		log.Println("H3: verifyPeerCertificate", certs)
		return nil
	}
	mtlsServerConfig.VerifyConnection = func(state tls.ConnectionState) error {
		log.Println("H3: verifyConnection state=", state)
		return nil
	}
	mtlsServerConfig.ClientAuth = tls.RequireAnyClientCert // only one supported by mint?

	qa.tlsServerConfig = mtlsServerConfig

	//quicServer := &http3.Server{
	//	QuicConfig: &quic.Config{
	//		MaxIdleTimeout: 60 * time.Second, // should be very large - but need to test recovery
	//		KeepAlive:      true,             // 1/2 idle timeout
	//		//Versions:    []quic.VersionNumber{101},
	//
	//		MaxIncomingStreams:    30000,
	//		MaxIncomingUniStreams: 30000,
	//
	//		EnableDatagrams: true,
	//	},
	//
	//	Server: &http.Server{
	//		Addr:        ":" + strconv.Itoa(qa.Port),
	//		Handler:     qa.HTTPHandler,
	//		TLSConfig:   mtlsServerConfig,
	//		ReadTimeout: 5 * time.Second,
	//	},
	//}
	//
	//qa.server = quicServer
	//quicServer.Init()

	return nil
}


// Either direction - h3 is indicated by a header frame.
// Format is: i(type) i(len) payload[len]
// Type: data(0), header(1),
func (q *Quic) handleRaw(qs quic.Stream) {
	str := nio2.GetStream(qs, qs)

	err := str.ReadHeader(qs)
	if err != nil {
		log.Println("Receive error ", err)
		return
	}

	str.Dest = str.InHeader.Get("dest")
	log.Println("QUIC stream IN", str.StreamId, str.Dest)

	str.PostDialHandler = func(conn net.Conn, err error) {
		str.Header().Set("status", "200")
		str.SendHeader(qs, str.Header())
		log.Println("QUIC stream IN rcv", str.StreamId, str.InHeader)
	}

	nc, err := q.Dialer.DialContext(qs.Context(), "tcp", str.Dest)
	if err != nil {
		str.PostDialHandler(nil, err)
		return
	}
	str.PostDialHandler(nil, nil)

	nio2.Proxy(nc, str, str, str.Dest)

}



func (q *Quic) quicConfig() *quic.Config {
	return &quic.Config{
		//RequestConnectionIDOmission: false,
		// should be very large - but need to test recovery

		//MaxIdleTimeout: 15 * time.Minute, // default 30s

		HandshakeIdleTimeout: 4 * time.Second, // default 10

		// make sure we don't get 0.
		//ConnectionIDLength: 4,

		MaxIncomingStreams:    30000,
		MaxIncomingUniStreams: 30000,

		//KeepAlive: true, // 1/2 idle timeout
		//Versions:  []quic.VersionNumber{quic.VersionDraft29},

		EnableDatagrams: true,

		// Increasing it 10x doesn't seem to change the speed.
		InitialStreamReceiveWindow:     4 * 1024 * 1024,
		MaxStreamReceiveWindow:         16 * 1024 * 1024,
		InitialConnectionReceiveWindow: 4 * 1024 * 1024,
		MaxConnectionReceiveWindow:     64 * 1024 * 1024,
	}
}
func (qd *Quic) DialContext(ctx context.Context, net, addr string) (net.Conn, error) {

	// Quic maintains 'associations' and mux connections.
	ugc := &QuicMUX{Addr: addr , client: true}

	err := qd.DialMux(ctx, ugc)
	if err != nil {
		return nil, err
	}
	return ugc.DialContext(ctx, net, addr)
}

func (qd *Quic) DialMux(ctx context.Context, ugc *QuicMUX) (error) {
	tlsConf := &tls.Config{
		// VerifyPeerCertificate used instead
		InsecureSkipVerify: true,

		Certificates: []tls.Certificate{*qd.Auth.Certificate},

		NextProtos: []string{"h3r", "h3-29"},
	}

	addr := ugc.Addr
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return err
	}

	// session.Context() is canceled when the session is closed.
	session, err := quic.DialEarly(ctx, udpConn, udpAddr, tlsConf, qd.quicConfig())
	if err != nil {
		return err
	}

	ugc.s = session

	//var rt http.RoundTripper
	// TODO: use node.WorkloadID if available - Addr is a temp address and may
	// be shared.
	//tok := qd.Auth.VAPIDToken(node.ID)

	//session.SendDatagram([]byte(tok))

	go func() {
		<-session.Context().Done()
		log.Println("H3: stop on ", qd.Auth.PubB32(), "for", ugc.Addr, session.RemoteAddr())
	}()

	//decoder := qpack.NewDecoder(nil)

	go func() {
		for {
			str, err := ugc.s.AcceptStream(context.Background())
			if err != nil {
				log.Println("H3: acceptStream err", err)
				return
			}
			go qd.handleRaw(str)
		}
	}()

	go qd.handleMessages(ugc)
	return  nil
}

func GetPort(a string, dp int32) int32 {
	if a == "" {
		return dp
	}
	_, p, err := net.SplitHostPort(a)
	if err != nil {
		return dp
	}
	pp, err := strconv.Atoi(p)
	if err != nil {
		return dp
	}
	return int32(pp)
}

func (qd *Quic) Start(ctx context.Context) error {
	c, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: int(GetPort(qd.Address, 8443)),
	})
	if err != nil {
		log.Println("H3: Failed to listen quic ", err)
		return err
	}

	l, err := quic.ListenEarly(c, qd.tlsServerConfig, qd.quicConfig())
	if err != nil {
		log.Println("H3: Failed to start server ", err)
		return err
	}

	go func() {
		for {
			s, err := l.Accept(context.Background())
			if err != nil {
				log.Println("H3: accept error", err)
				return
			}

			ugc := &QuicMUX{s: s, client: false}

			// QUIC supports 0-RTT, meaning that mTLS handshake may not complete until later.
			// JWT tokens can be used to find the identity.

			// not blocking
			qd.handleMessages(ugc)

			go func() {
				for {
					str, err := s.AcceptStream(context.Background())
					if err != nil {
						log.Println("AcceptStream err", err)
						return
					}
					go qd.handleRaw(str)
				}
			}()
		}
	}()

	return nil
}

// handleMessages is called for accepted messages (datagrams).
// Will call the cancel function when done.
func (qd *Quic) handleMessages(ugc *QuicMUX) {
	go func() {
		for {
			d, err := ugc.s.ReceiveDatagram(context.Background())
			if err != nil {
				log.Println("H3: ReceiveMessage err ", err)
				return
			}
			if len(d) == 0 {
				continue
			}
			log.Println("Received QUIC message", string(d))
		}
	}()
}
