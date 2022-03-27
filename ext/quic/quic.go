package quic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/auth"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
)

func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		qa := New(ug)

		return func(ug *ugatesvc.UGate) {
			qa.Start()
		}
	})
}


// Quic is the adapter to QUIC/H3/MASQUE for uGate.
//
// Implements:
// - MuxDialer
//
// Will start a H3 server and dispatch streams and H3 requests to uGate
//
// Integration with Quic library:
// - fork to expose few internal methods needed
// - low level UDP will be multiplexed with STUN/TURN - feature is supported upstream
// - using the raw QUIC library for streams, to get access to the reverse path
// - main RoundTripper is uGate, this acts as 'client.go'.
//
// TODO: Datagram will also be dispatched - either as UDP or as Webpush messages
// TODO: define 'webpush over MASQUE'
// TODO: also MASQUE-IP, if TUN support is enabled ( Android )
type Quic struct {
	Auth      *auth.Auth
	Port      int

	// Incoming streams are mapped to HTTP
	HTTPHandler http.Handler
	server      *http3.Server

	// UgateSVC - for node tracking
	UG              *ugatesvc.UGate
	tlsServerConfig *tls.Config
}

// DataSreamer is implemented by response writer on server side to take over the stream.
type DataStreamer interface {
	DataStream() quic.Stream
}

func New(ug *ugatesvc.UGate) *Quic {
	//os.Setenv("QUIC_GO_LOG_LEVEL", "DEBUG")

	// We will only register a single QUIC server by default, and a factory for cons
	port := ug.Config.BasePort + ugate.PORT_BTS
	//if os.Getuid() == 0 {
	//	port = 443
	//}

	qa := &Quic{
		Port: port,
		Auth: ug.Auth,
		HTTPHandler: ug.H2Handler,
		UG: ug,
	}

	ug.MuxDialers["quic"] = qa
	mtlsServerConfig :=qa.Auth.GenerateTLSConfigServer()

	// Overrides
	mtlsServerConfig.NextProtos = []string{"h3r","h3-34"}

	// called with ClientAuth is RequestClientCert or RequireAnyClientCert
	mtlsServerConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		certs := make([]*x509.Certificate, len(rawCerts))
		for i, certEntry := range rawCerts {
			certs[i], _ = x509.ParseCertificate(certEntry)
		}
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

	return qa
}

func (q *Quic) quicConfig() *quic.Config {
	return &quic.Config{
		//RequestConnectionIDOmission: false,
		// should be very large - but need to test recovery

		//MaxIdleTimeout: 15 * time.Minute, // default 30s

		HandshakeIdleTimeout: 4 * time.Second, // default 10

		// make sure we don't get 0.
		ConnectionIDLength: 4,

		MaxIncomingStreams:    30000,
		MaxIncomingUniStreams: 30000,

		KeepAlive: true, // 1/2 idle timeout
		//Versions:  []quic.VersionNumber{quic.VersionDraft29},

		EnableDatagrams: true,

		// Increasing it 10x doesn't seem to change the speed.
		InitialStreamReceiveWindow: 4 * 1024 * 1024,
		MaxStreamReceiveWindow: 16 * 1024 * 1024,
		InitialConnectionReceiveWindow: 4 * 1024 * 1024,
		MaxConnectionReceiveWindow:  64 * 1024 * 1024,
	}
}

func (qd *Quic) DialMux(ctx context.Context, node *ugate.DMNode, meta http.Header, ev func(t string, stream *ugate.Conn)) (ugate.Muxer, error) {
	tlsConf := &tls.Config{
		// VerifyPeerCertificate used instead
		InsecureSkipVerify: true,

		Certificates: []tls.Certificate{*qd.Auth.Cert},

		NextProtos: []string{"h3r", "h3-29"},
	}

	addr := node.Addr
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}



	// session.Context() is canceled when the session is closed.
	session, err := quic.DialEarlyContext(ctx, udpConn, udpAddr, addr, tlsConf, qd.quicConfig())
	if err != nil {
		return nil, err
	}

	var rt http.RoundTripper
	// TODO: use node.ID if available - Addr is a temp address and may
	// be shared.
	tok := qd.Auth.VAPIDToken(node.ID)

	session.SendMessage([]byte(tok))

	//if !UseRawStream {
	//	// Exposed in fork only.
	//	// Will call OpenUniStream, AcceptUniStream and OpenStream
	//	rt = http3.NewClient(node.ID, session, qd.quicConfig())
	//
	//	// TODO: use MASQUE ( with extension headers ? )
	//	initReq, _ := http.NewRequest("GET", "https://"+node.ID+"/_dm/id/Q/"+qd.Auth.ID, nil)
	//	initReq.Header.StartListener("authorization", tok)
	//	res, err := rt.RoundTrip(initReq)
	//	if err != nil {
	//		return nil, err
	//	}
	//	// TODO: parse the data, use MASQUE format for handshake
	//	_, _ = ioutil.ReadAll(res.Body)
	//	res.Body.Close()
	//
	//	if res != nil && res.StatusCode == 200 {
	//		log.Println("H3R: start on ", qd.Auth.ID, "for", node.ID, session.RemoteAddr())
	//	} else {
	//		log.Println("H3: start on ", qd.Auth.ID, "for", node.ID, session.RemoteAddr())
	//	}
	//}
	go func() {
		<- session.Context().Done()
		log.Println("H3: stop on ", qd.Auth.ID, "for", node.ID, session.RemoteAddr())
		node.Muxer = nil
		// TODO: anything to do on close ?
		qd.UG.OnMuxClose(node)
	}()

	ugc := &QuicMUX{s: session, rt: rt, hostname: node.ID, client: true}
	//decoder := qpack.NewDecoder(nil)

	go func() {
		for {
			str, err := ugc.s.AcceptStream(context.Background())
			if err != nil {
				log.Println("H3: acceptStream err", err)
				return
			}

			//if UseRawStream {
				go qd.handleRaw(str)
			//} else {
			//	// Treat the remote (server) originated stream as a H3 reverse request.
			//	// The server will process the session and dispatch to a handler.
			//	go qd.server.HandleRequest(session, str, decoder, func() {
			//		// Called when done
			//	})
			//}
		}
	}()

	go qd.handleMessages(ugc)
	node.Muxer = ugc
	return ugc, nil
}

// will eventually support the MASQUE protocol, multiplexing
// other proxy protocols over H3.

func (qd *Quic) Start() error {
	// TODO: MUX, same port as client side (for better STUN)

	c, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: qd.Port,
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


			//if UseRawStream {
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
			//} else {
			//	// TODO: we need a way to pass the mux on the first
			//	// request ( or the ID request ) to associate it with the node.
			//	// At this point we don't have the identity.
			//	// Currently we use x-quic-session as experiment.
			//	go func() {
			//		qd.server.HandleConn(s)
			//
			//		if ugc.n != nil {
			//			ugc.n.Muxer = nil
			//			log.Println("H3R: stop on ", qd.Auth.ID, "for", ugc.n.ID, s.RemoteAddr())
			//			qd.UG.OnMuxClose(ugc.n)
			//		} else {
			//			log.Println("H3: stop on ", qd.Auth.ID, s.RemoteAddr())
			//		}
			//
			//	}()
			//}

		}
	}()

	return nil
}

// handleMessages is called for accepted messages (datagrams).
// Will call the cancel function when done.
func (qd *Quic) handleMessages(ugc *QuicMUX) {
	go func() {
		var n *ugate.DMNode
		for {
			d, err := ugc.s.ReceiveMessage()
			if err != nil {
				log.Println("H3: ReceiveMessage err ", err)
				return
			}
			if len(d) == 0 {
				continue
			}
			if d[0] == 'v' { // 'vapid t= ,k=
				jwt := string(d)
				_, pub, err := auth.CheckVAPID(jwt, time.Now())
				if err == nil {
					remoteID := auth.IDFromPublicKeyBytes(pub)
					log.Println("H3R: start", remoteID, "on", qd.UG.Auth.ID)
					n = qd.UG.GetOrAddNode(remoteID)

					ugc.n = n
					ugc.hostname = n.ID
					//ugc.rt = http3.NewClient(remoteID, ugc.s, qd.quicConfig())

					n.Muxer = ugc
				}
				continue
			}
			// TODO: other message types
		}
	}()
}


