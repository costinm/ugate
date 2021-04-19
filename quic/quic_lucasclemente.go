package quic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/auth"
	//"github.com/pion/quic"

	"github.com/lucas-clemente/quic-go"
	h2quic "github.com/lucas-clemente/quic-go/http3"
)

// 2020/09:
// - still not merged mtls patch for HTTP3,
// - missing push
// - low level QUIC works great !!!

// Modified QUIC, using a hack specific to Android P2P to work around its limitations.
// Also adds instrumentation (expvar)

// v2: if wifi connection is DIRECT-, client will listen on 0xFF.. multicast on port+1.
//     AP: if destination zone is p2p, will use the MC address and port+1 when dialing
//         multiple connections may use different ports - MC is next port. Requires knowing
//         the dest is the AP - recorded during discovery.
//     AP: as server, if zone is p2p, use port+1 an MC.
//     AP: as client, same process - the GW will have a port+1

// v3: bypass QUIC and avoid the hack, create a dedicated UDP bridge.
//     should work with both h2 and QUIC, including envoy.
//     AP-client: connect to localhost:XXXX (one port per client). Ap client port different.
//     Client-AP: connect localhost:5221 (reserved).
//     AP listens on UDP:5222, Client on TCP/UDP 127.0.0.1:5221 and UDP :5220
// Need to implement wifi-like ACK for each packet - this seems to be the main problem
// with broadcast. A second problem is the power/bw.

/*
Low level quic:
- stream: StreamID, reader, cancelRead, SetReadDeadline
          writer+closer, CancelWrite, SetWriteDeadline
-

*/

/*
env variable for debug:
Mint:
- MINT_LOG=*|crypto,handshake,negotiation,io,frame,verbose

Client:
- QUIC_GO_LOG_LEVEL=debug|info|error
*/

/*
 Notes on the mint library:
 - supports AES-GCM with 12-bytes TAG, required by QUIC (aes12 packet)
 - fnv-1a hash - for older version (may be used in chrome), unprotected packets hash
 - quic-go-certificates - common compressed certs
 - buffer_pool.go - receive buffer pooled. Client also uses same
 -

	Code:
  - main receive loop server.go/serve() ->


  Packet:
   0x80 - long header = 1
	0x40 - has connection id, true in all cases for us


  Includes binaries for client-linux-debug from chrome (quic-clients)

  Alternative - minimal, also simpler: https://github.com/bifurcation/mint
  No h2, but we may not need this.
*/

// InitMASQUE will eventually support the MASQUE protocol, multiplexing
// proxy protocols over H3.
//
// However using a dedicated port achieves the same result.
func InitMASQUE(auth *auth.Auth, port int, handler ugate.ConHandler,
	conPool ugate.MuxConPool) error {
	c, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: port,
	})
	if err != nil {
		log.Println("H2: Failed to listen quic ", err)
		return err
	}

	mtlsServerConfig :=auth.GenerateTLSConfigServer()

	// called with ClientAuth is RequestClientCert or RequireAnyClientCert
	mtlsServerConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		certs := make([]*x509.Certificate, len(rawCerts))
		for i, certEntry := range rawCerts {
			certs[i], _ = x509.ParseCertificate(certEntry)
		}
		//if len(certs) > 0 {
		//	pub := certs[0].PublicKey.(*ecdsa.PublicKey)
		//	log.Println("SERVER TLS: ", len(certs), certs[0].DNSNames, pub)
		//}
		return nil
	}
	mtlsServerConfig.VerifyConnection = func(state tls.ConnectionState) error {
		log.Println(state)
		return nil
	}
	mtlsServerConfig.ClientAuth = tls.RequireAnyClientCert // only one supported by mint?

		QuicConfig:= &quic.Config{
			MaxIdleTimeout: 60 * time.Second, // should be very large - but need to test recovery
			KeepAlive:      true,             // 1/2 idle timeout
			//Versions:    []quic.VersionNumber{101},

			MaxIncomingStreams:    30000,
			MaxIncomingUniStreams: 30000,

			EnableDatagrams: true,
		}

	l, err := quic.Listen(c, mtlsServerConfig, QuicConfig)
	if err != nil {
		log.Println("H2: Failed to start server ", err)
		return err
	}
	go func() {
		for {
			s, err := l.Accept(context.Background())
			if err != nil {
				log.Println("MASQUE accept error", err)
				return
			}
			
			// s is a mux
			handleSession(s, handler, conPool)
			
		}
	}()
	
	return nil
}

type UGateCon struct {
	s quic.Session
}

func (ugs *UGateCon) SendMessage(d []byte) error {
	return ugs.s.SendMessage(d)
}

func (ugs *UGateCon) ReceiveMessage() ([]byte, error) {
	return ugs.s.ReceiveMessage()
}

func (ugs *UGateCon) DialContext(ctx context.Context, net, addr string) (net.Conn, error) {
	s, err := ugs.s.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	str := ugate.NewStream()
	str.In = s
	str.Out = s
	// TODO: populate metdata from session
	// TODO: add a header with metadata.
	return str, nil
}


func handleSession(s quic.Session, handler ugate.ConHandler, pool ugate.MuxConPool) {
	log.Println()

	ugc := &UGateCon{s: s}
	pool.OnConnect(ugc, "")
	go func() {
		for {
			str, err := s.AcceptStream(context.Background())
			if err != nil {
				log.Println("Quic session accept err", err)
				pool.OnDisconnect(ugc, "")
			}
			ustr := ugate.NewStream()
			ustr.In = str
			ustr.Out = str
			go handler.Handle(ustr)
		}
	}()

}


// InitQuicServer starts a regular QUIC server, bound to a port, using the H2 certificates.
func InitQuicServer(h2 *auth.Auth, port int, handler http.Handler) error {
	c, err := net.ListenUDP("udp", &net.UDPAddr{
			Port: port,
		})
	if err != nil {
		log.Println("H2: Failed to listen quic ", err)
		return err
	}

	err = InitQuicServerConn(h2, port, c, handler)
	if err != nil {
		log.Println("H2: Failed to start server ", err)
		return err
	}
	return nil
}

// InitQuicServerConn starts a QUIC server, using H2 certs, on a connection.
func InitQuicServerConn(auth *auth.Auth,port int, conn net.PacketConn, handler http.Handler) error {

	//conn = &telemetry.PacketConnWrapper{
	//	PacketConn: conn,
	//}

	mtlsServerConfig :=auth.GenerateTLSConfigServer()

	// called with ClientAuth is RequestClientCert or RequireAnyClientCert
	mtlsServerConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		certs := make([]*x509.Certificate, len(rawCerts))
		for i, certEntry := range rawCerts {
			certs[i], _ = x509.ParseCertificate(certEntry)
		}
		//if len(certs) > 0 {
		//	pub := certs[0].PublicKey.(*ecdsa.PublicKey)
		//	log.Println("SERVER TLS: ", len(certs), certs[0].DNSNames, pub)
		//}
		return nil
	}
	mtlsServerConfig.VerifyConnection = func(state tls.ConnectionState) error {
		log.Println(state)
		return nil
	}
	mtlsServerConfig.ClientAuth = tls.RequireAnyClientCert // only one supported by mint?

	quicServer := &h2quic.Server{
		QuicConfig: &quic.Config{
			MaxIdleTimeout: 60 * time.Second, // should be very large - but need to test recovery
			KeepAlive:      true,             // 1/2 idle timeout
			//Versions:    []quic.VersionNumber{101},

			MaxIncomingStreams:    30000,
			MaxIncomingUniStreams: 30000,

			EnableDatagrams: true,
		},

		Server: &http.Server{
			Addr:        ":" + strconv.Itoa(port),
			Handler:     handler,
			TLSConfig:   mtlsServerConfig,
			ReadTimeout: 5 * time.Second,
		},
	}
	go quicServer.Serve(conn)

	return nil
}

// Close the client in case of error. This actually closes all clients for any error - may
// need a separte wrapper per host. Original fix was to close only the bad client.
// h2quic implements Closer
type quicWrapper struct {
	Transport *h2quic.RoundTripper
}

func (qw *quicWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := qw.Transport.RoundTrip(req)
	if err != nil {
		//	slock.RLock()
		//	s, f := sessions[req.Host]
		//	slock.RUnlock()
		//	if f {
		//		log.Println("CLOSE SESSION HTTP error, closing client ", req.Host, req.URL, err)
		//		s.Close()
		//	} else {
		if err1, ok := err.(net.Error); ok && !err1.Timeout() {
			cl := io.Closer(qw.Transport) //.(io.Closer)
			if cl != nil {
				//slock.Lock()
				//delete(sessions, req.Host)
				//slock.Unlock()
				cl.Close()
				log.Println("HTTP error, closing client ", req.Host, req.URL, err)
			}
		} else if strings.Contains(err.Error(), "Crypto handshake did not") {
			cl := io.Closer(qw.Transport) //.(io.Closer)
			if cl != nil {
				//slock.Lock()
				//delete(sessions, req.Host)
				//slock.Unlock()
				cl.Close()
				log.Println("HTTP error, closing client ", err)
			}
		}
		//	}
	}

	return res, err
}

func RoundTripper(h2 *auth.Auth) http.RoundTripper {
	/*
		May 2018 - quic uses mint. client-state-machine implements the handshake.

		- without insecureSkipVerify, uses RootCAs, ServerName in x509 cert.Verify(VerifyOptions)
		- either way, calls VerifyPeerCertificate

	*/
	// tlsconfig.hostname can override the SNI
	//ctls.VerifyPeerCertificate = verify(destHost)
	ctls := h2.GenerateTLSConfigClient()
	qtorig := &h2quic.RoundTripper{
		//		Dial: h2.QuicDialer,

		TLSClientConfig: ctls,

		QuicConfig: &quic.Config{
			//RequestConnectionIDOmission: false,
			// should be very large - but need to test recovery

			MaxIdleTimeout: 15 * time.Minute, // default 30s

			HandshakeIdleTimeout: 4 * time.Second, // default 10

			// make sure we don't get 0.
			ConnectionIDLength: 4,

			MaxIncomingStreams:    30000,
			MaxIncomingUniStreams: 30000,

			KeepAlive: true, // 1/2 idle timeout
			//Versions:  []quic.VersionNumber{101},

			EnableDatagrams: true,
		},
		// holds a map of clients by hostname
	}
	qt1 := &quicWrapper{Transport: qtorig}
	return qt1
}

// InitQuicClient will configure h2.QuicClient as mtls
// using the h2 private key
// QUIC_GO_LOG_LEVEL
func InitQuicClient(h2 *auth.Auth, destHost string) *http.Client {

	qrtt := RoundTripper(h2)


	//if streams.MetricsClientTransportWrapper != nil {
	//	qrtt = streams.MetricsClientTransportWrapper(qrtt)
	//}

	//if UseQuic {
	//	if strings.Contains(host, "p2p") ||
	//			(strings.Contains(host, "wlan") && strings.HasPrefix(host, AndroidAPMaster)) {
	//		h2.quicClientsMux.RLock()
	//		if c, f := h2.quicClients[host]; f {
	//			h2.quicClientsMux.RUnlock()
	//			return c
	//		}
	//		h2.quicClientsMux.RUnlock()
	//
	//		h2.quicClientsMux.Lock()
	//		if c, f := h2.quicClients[host]; f {
	//			h2.quicClientsMux.Unlock()
	//			return c
	//		}
	//		c := h2.InitQuicClient()
	//		h2.quicClients[host] = c
	//		h2.quicClientsMux.Unlock()
	//
	//		log.Println("TCP-H2 QUIC", host)
	//		return c
	//	}
	//}

	return &http.Client{
		Timeout: 5 * time.Second,

		Transport: qrtt,
	}
}

//var (
//	// TODO: debug, clean, check, close
//	// has a Context
//	// ConnectionState - peer cert, ServerName
//	slock sync.RWMutex
//
//	// Key is Host of the request
//	sessions map[string]quic.Session = map[string]quic.Session{}
//)


//// Special dialer, using a custom port range, friendly to firewalls. From h2quic.RT -> client.dial()
//// This includes TLS handshake with the remote peer, and any TLS retry.
//func (h2 *H2) QuicDialer(network, addr string, tlsConf *tls.Config, config *quic.Config) (quic.EarlySession, error) {
//	udpAddr, err := net.ResolveUDPAddr("udp", addr)
//	if err != nil {
//		log.Println("QUIC dial ERROR RESOLVE ", qport, addr, err)
//		return nil, err
//	}
//
//	var udpConn *net.UDPConn
//	var udpConn1 *net.UDPConn
//
//	// We are calling the AP. Prepare a local address
//	if AndroidAPMaster == addr {
//		//// TODO: pool of listeners, etc
//		for i := 0; i < 10; i++ {
//			udpConn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
//			if err != nil {
//				continue
//			}
//			port := udpConn.LocalAddr().(*net.UDPAddr).Port
//			udpConn1, err = net.ListenMulticastUDP("udp6", AndroidAPIface,
//				&net.UDPAddr{
//					IP:   AndroidAPLL,
//					Port: port + 1,
//					Zone: AndroidAPIface.Name,
//				})
//			if err == nil {
//				break
//			} else {
//				udpConn.Close()
//			}
//		}
//
//		log.Println("QC: dial remoteAP=", addr, "local=", udpConn1.LocalAddr(), AndroidAPLL)
//
//	}
//
//	qport = 0
//	if udpConn == nil {
//		udpConn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
//	}
//
//	log.Println("QC: dial remote=", addr, "local=", udpConn.LocalAddr(), AndroidAPMaster, AndroidAPLL)
//	quicDialCnt.Add(1)
//
//	cw := &ClientPacketConnWrapper{
//		PacketConn: udpConn,
//		addr:       addr,
//		start:      time.Now(),
//	}
//	cw.useApHack = h2.Conf["p2p_multicast"] == "true"
//	if udpConn1 != nil {
//		cw.PacketConnAP = udpConn1
//	}
//	qs, err := quic.Dial(cw, udpAddr, addr, tlsConf, config)
//	if err != nil {
//		quicDialErrDial.Add(1)
//		quicDialErrs.Add(err.Error(), 1)
//		udpConn.Close()
//		if udpConn1 != nil {
//			udpConn1.Close()
//		}
//		return qs, err
//	}
//	slock.Lock()
//	sessions[addr] = qs
//	slock.Unlock()
//
//	go func() {
//		m := <-qs.Context().Done()
//		log.Println("QC: session close", addr, m)
//		slock.Lock()
//		delete(sessions, addr)
//		slock.Unlock()
//		udpConn.Close()
//		if udpConn1 != nil {
//			udpConn1.Close()
//		}
//	}()
//	return qs, err
//}

// --------  Wrappers around quic structs to intercept and modify the routing using multicast -----------

