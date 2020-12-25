package ugate

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// TLSConn extens tls.Conn with extra metadata.
// Adds the Proxy() method, implements ReadFrom and WriteTo using recycled buffers.
type TLSConn struct {
	// wrapps the original conn for Local/RemoteAddress and deadlines
	// Implements CloseWrite, ConnectionState,
	*tls.Conn
	RemotePub     crypto.PublicKey

	// Raw TCP connection, for remote address and stats
	// TODO: for H2-over-TLS-over-WS, it will be a WS conn
	Raw *Stats
}

func NewTLSConnOut(ctx context.Context, nc net.Conn, cfg *tls.Config, peerID string) (*TLSConn, error) {

	lc := &TLSConn{
	}
	if mc, ok := nc.(MetaConn); ok {
		lc.Raw = mc.Meta()
	} else {
		lc.Raw = &Stats{}
	}
	config, keyCh := ConfigForPeer(cfg, peerID)
	cs, cc, err := lc.handshake(ctx, tls.Client(nc, config), keyCh)
	lc.Raw.RemoteChain = cc
	lc.Conn = cs

	return lc, err
}


// SecureInbound runs the TLS handshake as a server.
// Accepts connections without client certificate - alternate form of auth will be used, either
// an inner TLS connection or JWT in metadata.
func NewTLSConnIn(ctx context.Context, nc net.Conn, cfg *tls.Config) (*TLSConn, error) {
	config, keyCh := ConfigForPeer(cfg, "")
	tc := &TLSConn{}
	if mc, ok := nc.(MetaConn); ok {
		tc.Raw = mc.Meta()
	} else {
		tc.Raw = &Stats{}
	}
	cs, cc, err := tc.handshake(ctx, tls.Server(nc, config), keyCh)
	tc.Conn= cs
	tc.Raw.RemoteChain = cc
	return tc, err
}

func (t *TLSConn) Meta() *Stats {
	return t.Raw
}

func (b *TLSConn) PostDial(nc net.Conn, err error) {
}

// The proxy can't be spliced - use regular write.
func (b *TLSConn) Proxy(nc net.Conn) error {
	errCh := make(chan error, 2)
	go b.proxyFromClient(nc, errCh)
	return b.proxyToClient(nc, errCh)
}

func (b *TLSConn) proxyToClient(cin io.Writer, errch chan error) error {
	b.WriteTo(cin) // errors are preserved in stats, 4 kinds possible

	// WriteTo doesn't close the writer !
	if c, ok := cin.(CloseWriter); ok {
		c.CloseWrite()
	}

	remoteErr := <- errch

	// The read part may have returned EOF, or the write may have failed.
	// In the first case close will send FIN, else will send RST
	b.Conn.Close()

	if c, ok := cin.(io.Closer); ok {
		c.Close()
	}

	return remoteErr
}

// WriteTo implements the interface, using the read buffer.
func (tc *TLSConn) WriteTo(w io.Writer) (n int64, err error) {
	buf1 := bufferPoolCopy.Get().([]byte)
	defer bufferPoolCopy.Put(buf1)
	bufCap := cap(buf1)
	buf := buf1[0:bufCap:bufCap]

	for {
		sn, sErr := tc.Conn.Read(buf)

		if sn > 0 {
			wn, wErr := w.Write(buf[0:sn])
			n += int64(wn)
			if wErr != nil {
				tc.Raw.ProxyWriteErr = wErr
				return n, wErr
			}
		}
		// May return err but still have few bytes
		if sErr != nil {
			sErr = eof(sErr)
			tc.Raw.ReadErr = sErr
			return n, sErr
		}
	}
}

// proxyFromClient writes to the net.Conn. Should be in a go routine.
func (b *TLSConn) proxyFromClient(cin io.Reader, errch chan error)  {
	_, err := b.ReadFrom(cin)

	// At this point either cin returned FIN or RST

	b.Conn.CloseWrite()
	errch <- err
}

// Reads data from cin (the client/dialed con) until EOF or error
// TCP Connections typically implement this, using io.Copy().
func (b *TLSConn) ReadFrom(cin io.Reader) (n int64, err error) {
	if wt, ok := cin.(io.WriterTo); ok {
		return wt.WriteTo(b.Conn)
	}

	buf1 := bufferPoolCopy.Get().([]byte)
	defer bufferPoolCopy.Put(buf1)
	bufCap := cap(buf1)
	buf := buf1[0:bufCap:bufCap]

	for {
		if srcc, ok := cin.(net.Conn); ok {
			srcc.SetReadDeadline(time.Now().Add(15 * time.Minute))
		}
		nr, er := cin.Read(buf)
		if er != nil {
			er = eof(err)
			return n, er
		}

		nw, err := b.Conn.Write(buf[0:nr])
		n += int64(nw)
		if err != nil {
			return n, err
		}
	}

	return
}

func ConfigForPeer(cfg *tls.Config, remotePeerID string) (*tls.Config, <-chan []*x509.Certificate) {
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

		chain, err := RawToCertChain(rawCerts)
		if err != nil {return err}

		pubKey, err := PubKeyFromCertChain(chain)
		pubKeyPeerID := IDFromPublicKey(pubKey)
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
		tlsConn *tls.Conn,
		keyCh <-chan []*x509.Certificate,
) (*tls.Conn, []*x509.Certificate, error) {

	// There's no way to pass a context to tls.Conn.Handshake().
	// See https://github.com/golang/go/issues/18482.
	// Close the connection instead.
	select {
	case <-ctx.Done():
		tlsConn.Close()
	default:
	}

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
			tlsConn.Close()
		}
	}()

	if err := tlsConn.Handshake(); err != nil {
		// if the context was canceled, return the context error
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, nil ,ctxErr
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

	return tlsConn, remotePubKey, nil
}
