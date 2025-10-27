package nio2

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// StreamConn wraps a net.Conn or a tls connection, implements net.Conn
type StreamConn struct {
	StreamState

	Conn net.Conn // may be a *tls.Conn

	// TLS info - if the connection is direct TLS.
	TLS *tls.ConnectionState

	// May be populated from Istio metadata or PROXY protocol, etc.
	ResponseHeader http.Header
	RequestHeaders http.Header

	ctx context.Context
}

func (s *StreamConn) Read(b []byte) (n int, err error) {
	return s.Conn.Read(b)
}

func (s *StreamConn) Write(b []byte) (n int, err error) {
	return s.Conn.Write(b)
}

func (s *StreamConn) Close() error {
	return s.Conn.Close()
}

func (s *StreamConn) LocalAddr() net.Addr {
	return s.Conn.LocalAddr()
}

func (s *StreamConn) RemoteAddr() net.Addr {
	return s.Conn.RemoteAddr()
}

func (s *StreamConn) SetDeadline(t time.Time) error {
	return s.Conn.SetDeadline(t)
}

func (s *StreamConn) SetReadDeadline(t time.Time) error {
	return s.Conn.SetReadDeadline(t)
}

func (s *StreamConn) SetWriteDeadline(t time.Time) error {
	return s.Conn.SetWriteDeadline(t)
}

func (s *StreamConn) State() *StreamState {
	return &s.StreamState
}

func (s *StreamConn) Header() http.Header {
	return s.ResponseHeader
}

func (s *StreamConn) RequestHeader() http.Header {
	return s.RequestHeaders
}

func (s *StreamConn) TLSConnectionState() *tls.ConnectionState {
	return s.TLS
}

func (s *StreamConn) Context() context.Context {
	return s.ctx
}

// NewStreamConn creates the Stream wrapper around a net.Conn
// If tls.Conn, will also set the TLS field (which can also be set for other
// streams ).
func NewStreamConn(r net.Conn) *StreamConn { // *StreamHttpServer {

	ss := &StreamConn{
		//StreamId: int(atomic.AddUint32(&nio.StreamId, 1)),
		StreamState: StreamState{Stats: Stats{Open: time.Now()}},
		Conn:        r,
	}
	if tc, ok := r.(*tls.Conn); ok {
		cs := tc.ConnectionState()
		ss.TLS = &cs
	}
	return ss
}

