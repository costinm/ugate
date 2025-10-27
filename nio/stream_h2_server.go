package nio

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// StreamHttpServer implements net.Conn on top of a H2 stream.
type StreamHttpServer struct {
	StreamState

	Request        *http.Request
	TLS            *tls.ConnectionState
	ResponseWriter http.ResponseWriter

	// If set, the function will be called when Close() is called.
	ReadCloser func()
}

func (s *StreamHttpServer) Context() context.Context {
	return s.Request.Context()
}

// Create a new stream from a HTTP request/response.
//
// For accepted requests, http2/server.go newWriterAndRequests populates the request based on the headers.
// Server validates method, path and scheme=http|https. Req.Body is a pipe - similar with what we use for egress.
// Request context is based on stream context, which is a 'with cancel' based on the serverConn baseCtx.
func NewStreamServerRequest(r *http.Request, w http.ResponseWriter) *StreamHttpServer { // *StreamHttpServer {
	return &StreamHttpServer{
		//StreamId: int(atomic.AddUint32(&nio.StreamId, 1)),
		StreamState: StreamState{Stats: Stats{Open: time.Now()}},

		Request:        r,
		ResponseWriter: w,
		// TODO: extract from JWT, reconstruct
		TLS: r.TLS,
		//Dest:    r.Host,
	}
}

func (s *StreamHttpServer) Read(b []byte) (n int, err error) {
	// TODO: update stats
	return s.Request.Body.Read(b)
}

func (s *StreamHttpServer) Write(b []byte) (n int, err error) {
	n, err = s.ResponseWriter.Write(b)
	if err != nil {
		s.WriteErr = err
		return n, err
	}
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
	s.SentBytes += n
	s.SentPackets++
	s.LastWrite = time.Now()

	return
}

func (s *StreamHttpServer) Close() error {
	if s.ReadCloser != nil {
		s.ReadCloser()
	}
	return s.CloseWrite()
}

func (s *StreamHttpServer) CloseWrite() error {
	// There is no real close - returning from the handler will be the close.
	// This is a problem for flushing and proper termination, if we terminate
	// the connection we also stop the reading side.
	// Server side HTTP stream. For client side, FIN can be sent by closing the pipe (or
	// request body). For server, the FIN will be sent when the handler returns - but
	// this only happen after request is completed and body has been read. If server wants
	// to send FIN first - while still reading the body - we are in trouble.

	// That means HTTP2 TCP servers provide no way to send a FIN from server, without
	// having the request fully read.
	// This works for H2 with the current library - but very tricky, if not set as trailer.

	// BUG: concurrent map write.
	// commented out for now
	//s.ResponseWriter.Header().Set("X-Close", "0")
	//s.ResponseWriter.(http.Flusher).Flush()
	return nil
}

func (s *StreamHttpServer) LocalAddr() net.Addr {
	//TODO implement me
	panic("implement me")
}

func (s *StreamHttpServer) RemoteAddr() net.Addr {
	if s.Request != nil && s.Request.RemoteAddr != "" {
		r, err := net.ResolveTCPAddr("tcp", s.Request.RemoteAddr)
		if err == nil {
			return r
		}
	}
	return nil
}

func (s *StreamHttpServer) SetDeadline(t time.Time) error {
	s.SetReadDeadline(t)
	return s.SetWriteDeadline(t)
}

func (s *StreamHttpServer) SetReadDeadline(t time.Time) error {
	return nil
}

func (s *StreamHttpServer) SetWriteDeadline(t time.Time) error {
	return nil
}

func (s *StreamHttpServer) State() *StreamState {
	return &s.StreamState
}

func (s *StreamHttpServer) Header() http.Header {
	return s.ResponseWriter.Header()
}

func (s *StreamHttpServer) RequestHeader() http.Header {
	return s.Request.Header
}

// TLSConnectionState implements the tls.Conn interface.
// By default uses the request TLS state, but can be replaced
// with a synthetic one (for example with ztunnel or other split
// TLS).
func (s *StreamHttpServer) TLSConnectionState() *tls.ConnectionState {
	return s.TLS
}
