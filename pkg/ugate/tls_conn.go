package ugate

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"sync"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/auth"
)

// TLSConn extens tls.Conn with extra metadata.
// Adds the Proxy() method, implements ReadFrom and WriteTo using recycled buffers.
type TLSConn struct {
	// Raw TCP connection, for remote address and stats
	// TODO: for H2-over-TLS-over-WS, it will be a WS conn
	*ugate.Stream

	// wrapps the original conn for Local/RemoteAddress and deadlines
	// Implements CloseWrite, ConnectionState,
	tls *tls.Conn
}

func (ug *UGate) NewTLSConnOut(ctx context.Context, nc net.Conn, cfg *tls.Config, peerID string, alpn []string) (*TLSConn, error) {
	lc := &TLSConn{
	}
	if mc, ok := nc.(ugate.MetaConn); ok {
		lc.Stream = mc.Meta()
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

	return lc, err
}

// SecureInbound runs the TLS handshake as a server.
// Accepts connections without client certificate - alternate form of auth will be used, either
// an inner TLS connection or JWT in metadata.
func (ug *UGate) NewTLSConnIn(ctx context.Context, nc net.Conn, cfg *tls.Config) (*TLSConn, error) {
	config, keyCh := ConfigForPeer(ug.Auth, cfg, "")
	config.NextProtos = []string{"h2r", "h2"}

	tc := &TLSConn{}
	tc.Stream = ugate.NewStream()
	if mc, ok := nc.(ugate.MetaConn); ok {
		m := mc.Meta()
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

	return tc, err
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
