package ugatesvc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/auth"
)

// TLSConn extends tls.Conn with extra metadata.
// Adds the Proxy() method, implements ReadFrom and WriteTo using recycled buffers.
type TLSConn struct {
	// Raw TCP connection, for remote address and stats
	// TODO: for H2-over-TLS-over-WS, it will be a WS conn
	*ugate.Stream

	// wrapps the original conn for Local/RemoteAddress and deadlines
	// Implements CloseWrite, ConnectionState,
	tls *tls.Conn
}

func (ug *UGate) NewTLSConnOut(ctx context.Context, nc net.Conn, cfg *tls.Config, peerID string, alpn []string) (*ugate.Stream, error) {
	lc := &TLSConn{
	}
	if mc, ok := nc.(*ugate.Stream); ok {
		lc.Stream = mc
		if rnc, ok := lc.Out.(net.Conn); ok {
			nc = rnc
		}
	} else {
		lc.Stream = ugate.NewStream()
	}

	config, keyCh := ConfigForPeer(ug.Auth,cfg, peerID)
	if alpn != nil {
		config.NextProtos = alpn
	}
	lc.tls = tls.Client(nc, config)
	cs, _, err := lc.handshake(ctx, keyCh)
	if err != nil {
		return nil, err
	}
	lc.Stream.In = lc.tls
	lc.Stream.Out = lc.tls

	//lc.tls.ConnectionState().DidResume
	tcs := lc.tls.ConnectionState()
	lc.Stream.TLS = &tcs
	lc.tls = cs

	return lc.Stream, err
}

// SecureInbound runs the TLS handshake as a server.
// Accepts connections without client certificate - alternate form of auth will be used, either
// an inner TLS connection or JWT in metadata.
func (ug *UGate) NewTLSConnIn(ctx context.Context, l *ugate.Listener, nc net.Conn, cfg *tls.Config) (*ugate.Stream, error) {
	config, keyCh := ConfigForPeer(ug.Auth, cfg, "")
	if l.ALPN == nil {
		config.NextProtos = []string{"h2r", "h2"}
	} else {
		config.NextProtos = l.ALPN
	}

	tc := &TLSConn{}
	tc.Stream = ugate.NewStream()
	if mc, ok := nc.(*ugate.Stream); ok {
		m := mc
		// Sniffed, etc
		tc.Listener = m.Listener
		tc.Dest = m.Dest
	}

	tc.tls = tls.Server(nc, config)
	cs, _, err := tc.handshake(ctx, keyCh)
	tc.tls = cs
	if err != nil {
		return nil, err
	}
	tcs := tc.tls.ConnectionState()
	tc.Stream.TLS = &tcs
	tc.Stream.In = tc.tls
	tc.Stream.Out = tc.tls

	return tc.Stream, err
}

func ConfigForPeer(a *auth.Auth, cfg *tls.Config, remotePeerID string) (*tls.Config, <-chan []*x509.Certificate) {
	keyCh := make(chan []*x509.Certificate, 1)
	// We need to check the peer ID in the VerifyPeerCertificate callback.
	// The tls.Config it is also used for listening, and we might also have concurrent dials.
	// Clone it so we can check for the specific peer ID we're dialing here.
	conf := cfg.Clone()
	// We're using InsecureSkipVerify, so the verifiedChains parameter will always be empty.
	// We need to parse the certificates ourselves from the raw certs.
	conf.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		defer close(keyCh)

		if remotePeerID == "" && len(rawCerts) == 0 {
			// Legacy infrastructure, client certs not transmitted.
			keyCh <- nil
			return nil
		}

		chain, err := auth.RawToCertChain(rawCerts)
		if err != nil {
			return err
		}

		pubKey, err := auth.PubKeyFromCertChain(chain)
		//pubKeyPeerID := auth.IDFromCert(chain)
		pubKeyPeerID := auth.IDFromPublicKey(pubKey)
		if err != nil {
			return err
		}

		// TODO: also verify the SAN (Istio and DNS style)

		if remotePeerID != "" &&
				remotePeerID != pubKeyPeerID {
			return errors.New("peer IDs don't match")
		}
		keyCh <- chain
		return nil
	}
	return conf, keyCh
}

func (pl *TLSConn) handshake(
		ctx context.Context,
		keyCh <-chan []*x509.Certificate,
) (*tls.Conn, []*x509.Certificate, error) {

	// There's no way to pass a context to tls.Conn.Handshake().
	// See https://github.com/golang/go/issues/18482.
	// Close the connection instead.

	done := make(chan struct{})
	var wg sync.WaitGroup

	// Ensure that we do not return before
	// either being done or having a context
	// cancellation.
	defer wg.Wait()
	defer close(done)

	wg.Add(1)

	go func() {
		defer wg.Done()
		select {
		case <-done:
		case <-ctx.Done():
			pl.tls.Close()
		}
	}()

	if err := pl.tls.Handshake(); err != nil {
		// if the context was canceled, return the context error
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, nil, ctxErr
		}
		return nil, nil, err
	}

	// Should be ready by this point, don't block.
	var remotePubKey []*x509.Certificate
	select {
	case remotePubKey = <-keyCh:
	default:
	}

	// Also:
	//tlsConn.ConnectionState().PeerCertificates
	//t.RemotePub = remotePubKey
	//
	//// At this point the BufferedCon unsecure connection can't be used.
	//t.Conn = tlsConn

	return pl.tls, remotePubKey, nil
}

var sniErr = errors.New("Invalid TLS")

type clientHelloMsg struct { // 22
	vers                uint16
	random              []byte
	sessionId           []byte
	cipherSuites        []uint16
	compressionMethods  []uint8
	nextProtoNeg        bool
	serverName          string
	ocspStapling        bool
	scts                bool
	supportedPoints     []uint8
	ticketSupported     bool
	sessionTicket       []uint8
	secureRenegotiation []byte
	alpnProtocols       []string
}

// TLS extension numbers
const (
	extensionServerName uint16 = 0
)


func ParseTLS(acc *ugate.Stream) (*clientHelloMsg,error) {
	buf, err := acc.Fill(5)
	if err != nil {
		return nil, err
	}
	typ := buf[0] // 22 3 1 2 0
	if typ != 22 {
		return nil, sniErr
	}
	vers := uint16(buf[1])<<8 | uint16(buf[2])
	if vers != 0x301 {
		log.Println("Version ", vers)
	}

	rlen := int(buf[3])<<8 | int(buf[4])
	if rlen > 4096 {
		log.Println("RLen ", rlen)
		return nil, sniErr
	}

	off := 5
	m := clientHelloMsg{}

	end := rlen + 5
	buf, err = acc.Fill(end)
	if err != nil {
		return nil, err
	}
	clientHello := buf[5:end]
	chLen := end - 5

	if chLen < 38 {
		log.Println("chLen ", chLen)
		return nil, sniErr
	}

	// off is the last byte in the buffer - will be forwarded

	m.vers = uint16(clientHello[4])<<8 | uint16(clientHello[5])
	// random: data[6:38]

	sessionIdLen := int(clientHello[38])
	if sessionIdLen > 32 || chLen < 39+sessionIdLen {
		log.Println("sLen ", sessionIdLen)
		return nil, sniErr
	}
	m.sessionId = clientHello[39 : 39+sessionIdLen]
	off = 39 + sessionIdLen

	// cipherSuiteLen is the number of bytes of cipher suite numbers. Since
	// they are uint16s, the number must be even.
	cipherSuiteLen := int(clientHello[off])<<8 | int(clientHello[off+1])
	off += 2
	if cipherSuiteLen%2 == 1 || chLen-off < 2+cipherSuiteLen {
		return nil, sniErr
	}

	//numCipherSuites := cipherSuiteLen / 2
	//m.cipherSuites = make([]uint16, numCipherSuites)
	//for i := 0; i < numCipherSuites; i++ {
	//	m.cipherSuites[i] = uint16(data[2+2*i])<<8 | uint16(data[3+2*i])
	//}
	off += cipherSuiteLen

	compressionMethodsLen := int(clientHello[off])
	off++
	if chLen-off < 1+compressionMethodsLen {
		return nil, sniErr
	}
	//m.compressionMethods = data[1 : 1+compressionMethodsLen]
	off += compressionMethodsLen

	if off+2 > chLen {
		// ClientHello is optionally followed by extension data
		return nil, sniErr
	}

	extensionsLength := int(clientHello[off])<<8 | int(clientHello[off+1])
	off = off + 2
	if extensionsLength != chLen-off {
		return nil, sniErr
	}

	for off < chLen {
		extension := uint16(clientHello[off])<<8 | uint16(clientHello[off+1])
		off += 2
		length := int(clientHello[off])<<8 | int(clientHello[off+1])
		off += 2
		if off >= end {
			return nil, sniErr
		}

		switch extension {
		case extensionServerName:
			d := clientHello[off : off+length]
			if len(d) < 2 {
				return nil, sniErr
			}
			namesLen := int(d[0])<<8 | int(d[1])
			d = d[2:]
			if len(d) != namesLen {
				return nil, sniErr
			}
			for len(d) > 0 {
				if len(d) < 3 {
					return nil, sniErr
				}
				nameType := d[0]
				nameLen := int(d[1])<<8 | int(d[2])
				d = d[3:]
				if len(d) < nameLen {
					return nil, sniErr
				}
				if nameType == 0 {
					m.serverName = string(d[:nameLen])
					// An SNI value may not include a
					// trailing dot. See
					// https://tools.ietf.org/html/rfc6066#section-3.
					if strings.HasSuffix(m.serverName, ".") {
						return nil, sniErr
					}
					break
				}
				d = d[nameLen:]
			}
		default:
			//log.Println("TLS Ext", extension, length)
		}

		off += length
	}

	// Does not contain port !!! Assume the port is 443, or map it.

	// TODO: unmangle server name - port, mesh node

	if m.serverName != "" {
		//destAddr := m.serverName + ":443"
		acc.Dest = m.serverName
	}
	acc.Type = ugate.ProtoTLS


	return &m, nil
}
