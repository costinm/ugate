package nio

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"strings"
	"syscall"
	"time"
)

// No longer used - broken up in smaller pieces, independent of mesh - nio package.

// InOutStream implements net.Conn using a tunneled, multi-layer protocol with metadata.
//
// The 'raw' connection is typically:
// - an accepted connection - In/RequestInPipe are the raw net.Conn - with sniffing and processing of SNI/TLS-SNI, SOCKS
// - a TLSConn, wrapping the accepted connection
// - HTTP2 RequestBody+ResponseWriter
//
// Metadata is extracted from the headers, SNI, SOCKS, Iptables.
// Example:
// - raw TCP connection
// - SOCKS - extracted dest host:port or IP:port
// - IPTables - extracted original DST IP:port
// - SNI - extracted 'Server Name' - port based on the listener port
// - TLS - peer certificates, SNI, ALPN
//
// Metrics and security info are also maintained.
//
// Implements net.Conn - but does not implement ConnectionState(), so the
// stream can be used with H2 library to create multiplexed H2 connections
// over the stream.
type InOutStream struct {
	StreamState

	// In - data from remote.
	//
	// - TCP or TLS net.Conn,
	// - a http request Body (stream mapped to a http accepted connection in a Handler)
	// - http response Body (stream mapped to a client http connection)
	// - a QUIC stream - accepted or dialed
	// - some other ReadCloser.
	//
	// Closing In without fully reading all data may result in RST.
	//
	// Normal process for close is to call CloseWrite (sending a FIN), read In fully
	// ( i.e. until remote FIN is received ) and call In.Close.
	// If In.Close is called before FIN was received the TCP stack may send a RST if more
	// data is received from the other end.
	In io.ReadCloser `json:"-"`

	// Out - send to remote.
	//
	// - an instance of net.Conn or tls.Conn - both implementing CloseWrite for FIN
	// - http.ResponseWriter - for accepted HTTP connections, implements CloseWrite
	// - a Pipe - for dialed HTTP connections, emulating DialContext behavior ( no body sent before connection is
	//   completed)
	// - nil, if the remote side is read only ( GET ) or if the creation of the
	//   stream passed a Reader object which is automatically piped to the RequestInPipe, for example
	//   when a HTTP request body is used.
	//
	Out io.Writer `json:"-"`

	// Request associated with the stream. Will be set if the stream is
	// received over HTTP (real or over another virtual connection),
	// or if the stream is originated locally and sent to a HTTP dest.
	//
	// For streams associated with HTTP server handlers, Out is the ResponseWriter.
	//
	Request *http.Request `json:"-"`
	// ---------------------------------------------------
	// Used for gRPC and other similar framing protocols
	// Response holds the response, for client mode
	Response *http.Response

	// Metadata to send. InOutStream implements http.ResponseWriter.
	// For streams without metadata - will be ignored.
	// Incoming metadata is set in Request.
	// TODO: without a request, use a buffer, append headers in serialized format directly, flush on first Write
	// @Deprecated - use a buf.
	OutHeader http.Header `json:"-"`

	// Header received from the remote.
	// For egress it is the response headers.
	// For ingress it is the request headers.
	// TODO: map[int][]byte, use read buffer to parse to avoid alloc.
	// Use equivalent of QPACK with uncompressed headers, custom dict.
	// @Deprecated - use a buf, packed format, id-based headers.
	InHeader http.Header `json:"-"`

	// Set if the connection finished a TLS handshake.
	// A 'dummy' value may be set if a sidecar terminated the connection.
	TLS *tls.ConnectionState `json:"-"`

	// Remote mesh ID, if authenticated. Base32(SHA256(PUB)) or Base32(PUB) (for ED)
	// This can be used in DNS names, URLs, etc.
	RemoteID string

	// Remote mesh ID, in byte form.
	Remote [32]byte

	// VIP is the internal ID used in dmesh, based on the SHA of address or public key.
	RemoteVIP uint64

	// Original dest - hostname or IP, including port. Parameter of the original RoundTripStart from the captured egress stream.
	// May be a mesh IP6, host, etc. If original address was captured by IP, destIP will also be set.
	// Host is extracted from metadata (SOCKS, iptables, etc)
	// Typically a DNS or IP address
	// For example in CONNECT it will be hostname:port or IP:port
	// For HTTP PROXY the path is a full URL.
	Dest string

	LocalA net.Addr

	// Resolved destination IP. May be nil if SOCKS or forwarding is done. Final Gateway will have it set.
	// If capture is based on IP, it'll be set in all hops.
	// If set, this is the authoritiative destination.
	DestAddr *net.TCPAddr

	//  Real remote address form the socket. May be different from DestAddr (DestAddr can be VIP)
	RemoteA net.Addr

	// Client type - original capture and all transport hops.
	// SOCKS, CONNECT, PROXY, SOCKSIP, PROXYIP,
	// EPROXY = TCP-over-HTTP in, direct host out
	// MUX- - for streams associated with a mux.
	// TODO: use int
	Type string

	// ---------------------

	// Methods to call when the stream is closed on the read side, i.e. received a FIN or RST or
	// the context was canceled.
	ReadCloser func() `json:"-"`

	// Set if CloseWrite() was called, which should result in a FIN sent.
	// This should happen if a EOF was received when proxying.
	ServerClose bool `json:"-"`

	// Set if the client has sent the FIN, and gateway sent the FIN to server
	ClientClose bool `json:"-"`

	// Set if Close() was called.
	Closed bool `json:"-"`

	// Errors associated with this stream, read from or write to.

	// Only for 'accepted' streams (server side), in proxy mode: keep track
	// of the client side. The server is driving the proxying.
	ProxyReadErr  error `json:"-"`
	ProxyWriteErr error `json:"-"`

	// Context and cancel funciton for this stream.
	ctx context.Context `json:"-"`

	// Close will invoke this method if set, and cancel the context.
	ctxCancel context.CancelFunc `json:"-"`

	// Optional function to call after dial (proxied streams) or after a stream handling has started for local handlers.
	// Used to send back metadata or finish the handshake.
	//
	// For example in SOCKS it sends back the IP/port of the remote.
	// net.Conn may be a InOutStream or a regular TCP/TLS connection.
	PostDialHandler func(net.Conn, error) `json:"-"`

	//
	//
	//
	Direction StreamType

	// ---------------------------------------------------------
	// If not nil, this stream has a read buffer attached.
	//
	// Read methods will take the rbuffer into account, if present.
	// Buffers can also be detached and passed to other streams, for less copy.
	rbuffer *Buffer

	// If not nil, this stream has write buffer attached.
	wbuffer *Buffer

	Listener net.Listener

	// Settings
	//Cluster http.RoundTripper

	// Used for framing responses
	// lastFrame holds the last received frame. It will be reused by Recv if not nil.
	// The caller can take ownership of the frame by setting this to nil.
	lastFrame *Buffer

	// Will receive a 'nil' or error on connect.
	// Will receive a nil or error on receive error (clean close or RST)
	ErrChan chan error

	// internal channel for roundtrip, for Recv thread to sync with response getting
	// received.
	rtCh chan error
}

func (s *InOutStream) State() *StreamState {
	return &s.StreamState
}

type StreamType int

// If true, will debug or close operations.
// Close is one of the hardest problems, due to FIN/RST multiple interfaces.
const DebugClose = true

const (
	StreamTypeUnknown StreamType = iota

	// Ingress - received on the HBONE mux for the local process, on
	//  a 'sidecar'.
	StreamTypeIn

	// Egress - indicates if is originated from local machine, i.e.
	// SOCKS/iptables/TUN capture or dialed from local process
	StreamTypeOut

	// Forward - received on HBONE mux to forward to a workload
	StreamTypeForward
)

// --------- Buffering and sniffing --------------
// TODO: benchmark different sizes.

// GetStream should be used to get a (recycled) stream.
// Streams will be tracked, and must be closed and recycled.
func GetStream(out io.Writer, in io.ReadCloser) *InOutStream {
	s := NewStream()
	s.In = in
	s.Out = out
	return s
}

// RBuffer method will return or create a buffer. It can be used for parsing
// headers or sniffing. The 'Read' and 'WriteTo' methods are aware of the
// buffer, and will use the first consume buffered data, but if the buffer is
// IsEmpty will use directly In.
func (s *InOutStream) RBuffer() *Buffer {
	if s.rbuffer != nil {
		return s.rbuffer
	}
	br := GetBuffer(0, 0)
	s.rbuffer = br

	return s.rbuffer
}

// WBuffer returns the write buffer associated with the stream.
// Used to encode headers or for buffering - to avoid the pattern of allocating
// small non-pooled buffers.
// TODO: also to use for bucket passing instead of copy.
func (s *InOutStream) WBuffer() *Buffer {
	if s.wbuffer != nil {
		return s.wbuffer
	}
	br := GetBuffer(0, 0)
	s.wbuffer = br

	return s.wbuffer
}

// Fill the buffer by doing one Read() from the underlying reader.
//
// Future calls to Read() will use the remaining data in the buffer.
// @Deprecated - use BufferReader.Peek()
func (s *InOutStream) Fill(nb int) ([]byte, error) {
	b := s.RBuffer()
	return b.Fill(s.In, nb) // s.In.Read(b.buf[b.end:])
}

// Skip only implemented for buffer
// Deprecated: use BufferReader.Discard()
func (s *InOutStream) Skip(n int) {
	b := s.rbuffer
	n -= b.Skip(n)
	if n > 0 {
		// Now need to read and skip n
		for {
			bb, err := s.Fill(0)
			if err != nil {
				return
			}
			if len(bb) < n {
				n -= len(bb)
				b.Compact()
				continue
			} else if len(bb) == n {
				b.Compact()
				return
			} else {
				b.Skip(n)
				return
			}
		}
	}
}

func (s *InOutStream) ReadByte() (byte, error) {
	b := s.RBuffer()
	if b.IsEmpty() {
		_, err := s.Fill(0)
		if err != nil {
			return 0, err
		}
	}
	return b.ReadByte(s.In)
}

// ----------------------------------------------

// NewStream create a new stream. This stream is not tracked.
func NewStream() *InOutStream {
	return &InOutStream{
		//StreamId: int(atomic.AddUint32(&nio.StreamId, 1)),
		StreamState: StreamState{Stats: Stats{Open: time.Now()}},
	}
}

//func (s *InOutStream) Reset() {
//	s.Open = time.Now()
//	s.LastRead = time.Time{}
//	s.LastWrite = time.Time{}
//
//	s.RcvdBytes = 0
//	s.SentBytes = 0
//	s.RcvdPackets = 0
//	s.SentPackets = 0
//
//	s.ReadErr = nil
//	s.WriteErr = nil
//	s.Type = ""
//}

const ContextKey = "ugate.stream"

// DO NOT IMPLEMENT: H2 will use the ConnectionStater interface to
// detect TLS, and do checks. Would break plain text streams.
// Also auth is more flexibile then mTLS.
//// Used by H2 server to populate TLS in accepted requests.
//// For 'fake' TLS (raw HTTP) it must be populated.
//func (s *InOutStream) ConnectionState() tls.ConnectionState {
//	if s.TLS == nil {
//		return tls.ConnectionState{Version: tls.VersionTLS12}
//	}
//	return *s.TLS
//}

// Context of the stream. It has a value 'ugate.stream' that
// points back to the stream, so it can be passed in various
// methods that only take context.
//
// This is NOT associated with the context of the original H2 request,
// there is a lot of complexity and strange behaviors in the stack.
func (s *InOutStream) Context() context.Context {
	//if s.Request != nil {
	//	return s.Request.Context()
	//}
	if s.ctx == nil {
		s.ctx, s.ctxCancel = context.WithCancel(context.Background())
		s.ctx = context.WithValue(s.ctx, ContextKey, s)
	}
	return s.ctx
}

// Write implements the io.Writer. The Write() is flushed if possible.
//
// TODO: incorporate the wbuffer, optimize based on size.
func (s *InOutStream) Write(b []byte) (n int, err error) {
	n, err = s.Out.Write(b)
	if err != nil {
		s.WriteErr = err
		return n, err
	}
	s.SentBytes += n
	s.SentPackets++
	s.LastWrite = time.Now()
	if f, ok := s.Out.(http.Flusher); ok {
		f.Flush()
	}

	return
}

func (s *InOutStream) Flush() {
	// TODO: take into account the write buffer.
	if f, ok := s.Out.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *InOutStream) Read(out []byte) (int, error) {
	if s.rbuffer != nil {
		// Duplicated - the other method may be removed.
		b := s.rbuffer
		if b.Size() > 0 {
			n, err := b.ReadData(out)
			s.RcvdBytes += n
			s.RcvdPackets++
			return n, err
		}
	}
	n, err := s.In.Read(out)

	s.RcvdBytes += n
	s.RcvdPackets++
	s.LastRead = time.Now()

	if err != nil {
		s.ReadErr = err
		if s.ReadCloser != nil {
			s.ReadCloser()
			s.ReadCloser = nil
		}
	}
	return n, err
}

// Must be called at the end. It is expected CloseWrite has been called, for graceful FIN.
func (s *InOutStream) Close() error {
	if s.Closed {
		return nil
	}
	s.Closed = true
	if !s.ServerClose {
		if DebugClose {
			log.Println(s.StreamId, "Close without out.close() ", s.Dest, s.InHeader)
		}
		// For HTTP - this also happens in cleanup, after response is done.
		//s.CloseWrite()
	}

	if s.ctxCancel != nil {
		s.ctxCancel()
	}

	if s.rbuffer != nil {
		defer func() {
			s.rbuffer.Recycle()
			s.rbuffer = nil
		}()
	}
	if s.wbuffer != nil {
		defer func() {
			s.wbuffer.Recycle()
			s.wbuffer = nil
		}()
	}
	if DebugClose {
		log.Println(s.StreamId, "Close(in) ", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
	}
	return s.In.Close()
}

func (s *InOutStream) CloseWrite() error {
	if s.ServerClose {
		log.Println("Double CloseWrite")
		return nil
	}
	s.ServerClose = true

	if cw, ok := s.Out.(CloseWriter); ok {
		if DebugClose {
			log.Println(s.StreamId, "CloseWriter", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
		}
		return cw.CloseWrite()
	} else {
		if c, ok := s.Out.(io.Closer); ok {
			if DebugClose {
				log.Println(s.StreamId, "CloseWrite using RequestInPipe.Close()", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
			}
			return c.Close()
		} else {
			if rw, ok := s.Out.(http.ResponseWriter); ok {
				// Server side HTTP stream. For client side, FIN can be sent by closing the pipe (or
				// request body). For server, the FIN will be sent when the handler returns - but
				// this only happen after request is completed and body has been read. If server wants
				// to send FIN first - while still reading the body - we are in trouble.

				// That means HTTP2 TCP servers provide no way to send a FIN from server, without
				// having the request fully read.
				if DebugClose {
					log.Println(s.StreamId, "CloseWrite using HTTP trailer ", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
				}
				// This works for H2 with the current library - but very tricky, if not set as trailer.
				rw.Header().Set("X-Close", "0")
				rw.(http.Flusher).Flush()
			} else {
				log.Println("Server out not Closer nor CloseWriter nor ResponseWriter")
			}
		}
	}
	return nil
}

func (s *InOutStream) SetDeadline(t time.Time) error {
	s.SetReadDeadline(t)
	return s.SetWriteDeadline(t)
}

func (s *InOutStream) SetReadDeadline(t time.Time) error {
	if cw, ok := s.Out.(net.Conn); ok {
		cw.SetReadDeadline(t)
	}
	return nil
}

func (s *InOutStream) SetWriteDeadline(t time.Time) error {
	if cw, ok := s.Out.(net.Conn); ok {
		cw.SetWriteDeadline(t)
	}
	return nil
}

func (s *InOutStream) Header() http.Header {
	if rw, ok := s.Out.(http.ResponseWriter); ok {
		return rw.Header()
	}
	if s.OutHeader == nil {
		s.OutHeader = map[string][]string{}
	}
	return s.OutHeader
}

func (s *InOutStream) RequestHeader() http.Header {
	return s.Request.Header
}

func (s *InOutStream) TransportConn() net.Conn {
	return s.Out.(net.Conn)
}

func (s *InOutStream) TLSConnectionState() *tls.ConnectionState {
	if tc, ok := s.TransportConn().(*tls.Conn); ok {
		cs := tc.ConnectionState()
		return &cs
	}
	return nil
}

func (s *InOutStream) WriteHeader(statusCode int) {
	if rw, ok := s.Out.(http.ResponseWriter); ok {
		rw.WriteHeader(statusCode)
		return
	}
}

// Copy src to dst, using a pooled intermediary buffer.
//
// Will update stats about activity and data.
// Does not close dst when src is closed
//
// Blocking, returns when src returned an error or EOF/graceful close.
// May also return with error if src or dst return errors.
//
// srcIsRemote indicates that the connection is from the server to client. (remote to local)
// If false, the connection is from client to server ( local to remote )
func (s *InOutStream) CopyBuffered(dst io.Writer, src io.Reader, srcIsRemote bool) (written int64, err error) {
	buf1 := GetDataBufferChunk(16 * 64 * 1024)
	defer PutDataBufferChunk(buf1)
	bufCap := cap(buf1)
	buf := buf1[0:bufCap:bufCap]

	// For netstack: src is a gonet.ReaderCopier, doesn't implement WriterTo. Dst is a net.TcpConn - and
	//  implements ReadFrom.

	// Copy is the actual implementation of Copy and CopyBuffer.
	// if buf is nil, one is allocated.
	// Duplicated from io

	// This will prevent stats from working.
	// If the reader has a WriteTo method, use it to do the copy.
	// Avoids an allocation and a copy.
	//if wt, ok := src.(io.WriterTo); ok {
	//	return wt.WriteTo(dst)
	//}
	// Similarly, if the writer has a ReadFrom method, use it to do the copy.
	//if rt, ok := dst.(io.ReaderFrom); ok {
	//	return rt.ReadFrom(src)
	//}

	for {
		if srcc, ok := src.(net.Conn); ok {
			srcc.SetReadDeadline(time.Now().Add(15 * time.Minute))
		}
		nr, er := src.Read(buf)
		if er != nil && er != io.EOF {
			if strings.Contains(er.Error(), "NetworkIdleTimeout") {
				return written, io.EOF
			}
			return written, err
		}
		if nr == 0 {
			// shouldn't happen unless err == io.EOF
			return written, io.EOF
		}
		if nr > 0 {
			if srcIsRemote {
				s.LastRead = time.Now()
				s.RcvdPackets++
				s.RcvdBytes += int(nr)
			} else {
				s.SentPackets++
				s.SentBytes += int(nr)
				s.LastWrite = time.Now()
			}
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if f, ok := dst.(http.Flusher); ok {
				f.Flush()
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil { // == io.EOF
			return written, er
		}
	}
	return written, err
}

// Send will marshall the metadata (headers) and start sending the body to w.
func (s *InOutStream) SendHeader(w io.WriteCloser, h http.Header) error {
	// First format: TAG(=2), 4B LEN, Text headers. Required len, buffer

	bb := s.WBuffer()

	for k, vv := range h {
		for _, v := range vv {
			bb.WriteByte(1)
			bb.WriteVarint(int64(len(k)))
			bb.Write([]byte(k))
			bb.WriteVarint(int64(len(v)))
			bb.Write([]byte(v))
		}
	}
	bb.WriteByte(0)

	bb.WriteByte(2) // To differentiate from regular H3, using 0
	bb.Write([]byte{0, 0, 0, 0})
	err := s.OutHeader.Write(bb)
	binary.LittleEndian.PutUint32(bb.Bytes()[1:], uint32(bb.Size()-5))
	if err != nil {
		return err
	}
	_, err = w.Write(bb.Bytes())
	if err != nil {
		return err
	}
	if DebugClose {
		log.Println("InOutStream.sendHeaders ", s.StreamId, h)
	}
	return nil
}

func (s *InOutStream) ReadHeader(in io.Reader) error {
	// TODO: move to buffered stream, unify
	buf1 := GetDataBufferChunk(16 * 64 * 1024)
	defer PutDataBufferChunk(buf1)
	bufCap := cap(buf1)
	buf := buf1[0:bufCap:bufCap]

	n, err := io.ReadFull(in, buf[0:5])
	len := binary.LittleEndian.Uint32(buf[1:])
	if len > 32*1024 {
		return errors.New("header size")
	}
	n, err = io.ReadFull(in, buf[0:len])
	if err != nil {
		return err
	}
	hr := textproto.NewReader(bufio.NewReader(bytes.NewBuffer(buf[0:n])))
	mh, err := hr.ReadMIMEHeader()
	s.InHeader = http.Header(mh)

	if DebugClose {
		log.Println("InOutStream.receiveHeaders ", s.StreamId, s.InHeader)
	}
	return nil
}

func (s *InOutStream) LocalAddr() net.Addr {
	if s.LocalA != nil {
		return s.LocalA
	}
	if cw, ok := s.Out.(net.Conn); ok {
		return cw.LocalAddr()
	}

	if s.Listener != nil {
		return s.Listener.Addr()
	}
	return nil
}

// RemoteAddr is the client (for accepted) or server (for originated).
// It should be the real IP, extracted from connection or metadata.
// RemoteID returns the authenticated ID.
func (s *InOutStream) RemoteAddr() net.Addr {
	if s.RemoteA != nil {
		return s.RemoteA
	}
	// non-test Streams are either backed by a net.InOutStream or a Request
	if cw, ok := s.Out.(net.Conn); ok {
		return cw.RemoteAddr()
	}

	if s.Request != nil && s.Request.RemoteAddr != "" {
		r, err := net.ResolveTCPAddr("tcp", s.Request.RemoteAddr)
		if err == nil {
			return r
		}
	}

	// Only for dialed connections - first 2 cases should happen most of the
	// time for accepted connections.
	//log.Println("RemoteAddr fallback", s)
	if s.DestAddr != nil {
		return s.DestAddr
	}
	return nameAddress(s.Dest)
	// RoundTripStart doesn't set it very well...
	//return tp.SrcAddr
}

// Reads data from cin (the client/dialed con) until EOF or error
// TCP Connections typically implement this, using io.Copy().
func (s *InOutStream) ReadFrom(cin io.Reader) (n int64, err error) {
	// Typical case for accepted connections, TCPConn  implements
	// this efficiently by splicing.
	// TCP conn ReadFrom fallbacks to Copy without recycling the buffer
	if CanSplice(cin, s.Out) {
		if wt, ok := s.Out.(io.ReaderFrom); ok {
			//VarzReadFromC.Add(1)
			n, err = wt.ReadFrom(cin)
			s.SentPackets++
			s.SentBytes += int(n)
			return
		}
	}

	buf1 := GetDataBufferChunk(16 * 64 * 1024)
	defer PutDataBufferChunk(buf1)
	bufCap := cap(buf1)
	buf := buf1[0:bufCap:bufCap]

	for {
		// TODO: respect cluster timeouts !
		if srcc, ok := cin.(net.Conn); ok {
			srcc.SetReadDeadline(time.Now().Add(15 * time.Minute))
		}
		nr, er := cin.Read(buf)
		if nr > 0 {
			nw, err := s.Out.Write(buf[0:nr])
			n += int64(nw)
			s.SentBytes += nw
			s.SentPackets++
			if f, ok := s.Out.(http.Flusher); ok {
				f.Flush()
			}
			if err != nil {
				return n, err
			}
			if er != nil {
				s.ProxyReadErr = er
				return n, er
			}
		}
	}

	return
}

func (b *InOutStream) PostDial(nc net.Conn, err error) {
	if b.PostDialHandler != nil {
		b.PostDialHandler(nc, err)
	}
}

// If true, will debug or close operations.
// Close is one of the hardest problems, due to FIN/RST multiple interfaces.
//const DebugClose = true

// Proxy the accepted connection to a dialed connection.
// Blocking, will wait for both sides to FIN or RST.
func (s *InOutStream) ProxyTo(nc net.Conn) error {
	errCh := make(chan error, 2)
	go s.proxyFromClient(nc, errCh)

	// Blocking, returns when all data is read from In, or error
	var err1 error

	// Special case - the dialed connection is a Conn, and it has an nil RequestInPipe field -
	// this is used with a http.Request without using pipe.
	// Deprecated, no longer used- the plan is to change the h2 stack, pipe is not the biggest problem...
	//if ncs, ok := nc.(*Conn); ok {
	//	if ncs.RequestInPipe != nil {
	//		err1 = s.proxyToClient(nc)
	//	}
	//	// TODO: we need to wait for the request to consume the stream.
	//} else {

	err1 = s.proxyToClient(nc)

	// Wait for data to be read from nc and sent to RequestInPipe, or error
	remoteErr := <-errCh
	if remoteErr == nil {
		remoteErr = err1
	}

	// The read part may have returned EOF, or the write may have failed.
	// In the first case close will send FIN, else will send RST
	if DebugClose {
		log.Println(s.StreamId, "proxyTo ", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
	}
	s.In.Close()
	nc.Close()
	return remoteErr
}

// Read from the Reader, send to the cout client.
// Updates ReadErr and ProxyWriteErr
func (s *InOutStream) proxyToClient(cout io.WriteCloser) error {
	s.WriteTo(cout) // errors are preserved in stats, 4 kinds possible

	// At this point an error or graceful EOF from our Reader has been received.
	err := s.ProxyWriteErr
	if err == nil {
		err = s.ReadErr
	}

	if NoEOF(err) != nil {
		// Should send RST if unbuffered data (may also be FIN - no way to control)
		if DebugClose {
			log.Println(s.StreamId, "proxyToClient RST", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
		}
		cout.Close()
		s.In.Close()
	} else {
		// WriteTo doesn't close the writer ! We need to send a FIN, so remote knows we're done.
		if c, ok := cout.(CloseWriter); ok {
			if DebugClose {
				log.Println(s.StreamId, "proxyToClient EOF", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
			}
			s.ClientClose = true
			c.CloseWrite()
		} else {
			//if debugClose {
			log.Println(s.StreamId, "proxyToClient EOF, XXX Missing CloseWrite", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
			//}
			cout.Close()
		}
		// EOF was received already for normal close.
		// If a write error happened - we want to close it to force a RST.
		//if cc, ok := s.In.(CloseReader); ok {
		//	if debugClose {
		//		log.Println("proxyToClient CloseRead", s.StreamId, s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
		//	}
		//	cc.CloseRead()
		//}
	}
	return err
}

// WriteTo implements the interface, using the read buffer.
func (s *InOutStream) WriteTo(w io.Writer) (n int64, err error) {
	// Finish up the buffer first
	if s.rbuffer != nil && !s.rbuffer.IsEmpty() {
		b := s.rbuffer
		bn, err := w.Write(b.Bytes())
		if err != nil {
			//"Write must return non-nil if it doesn't write the full buffer"
			s.ProxyWriteErr = err
			return int64(bn), err
		}
		b.Skip(bn)
		n += int64(bn)
	}

	if CanSplice(s.In, w) {
		if wt, ok := w.(io.ReaderFrom); ok {
			//VarzReadFromC.Add(1)
			n, err = wt.ReadFrom(s.In)
			s.RcvdPackets++
			s.RcvdBytes += int(n)
			s.LastRead = time.Now()
			return
		}
	}

	var buf1 []byte
	if s.rbuffer != nil {
		buf1 = s.rbuffer.Bytes()
	} else {
		buf1 := GetDataBufferChunk(16 * 64 * 1024)
		defer PutDataBufferChunk(buf1)
	}
	bufCap := cap(buf1)
	buf := buf1[0:bufCap:bufCap]

	for { // TODO: respect cluster timeouts !
		if srcc, ok := s.In.(net.Conn); ok {
			srcc.SetReadDeadline(time.Now().Add(15 * time.Minute))
		}
		sn, sErr := s.In.Read(buf)
		s.RcvdPackets++
		s.RcvdBytes += sn

		if sn > 0 {
			wn, wErr := w.Write(buf[0:sn])
			n += int64(wn)
			if wErr != nil {
				s.ProxyWriteErr = wErr
				return n, wErr
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		// May return err but still have few bytes
		if sErr != nil {
			s.ReadErr = sErr
			return n, sErr
		}
	}
}

func NoEOF(err error) error {
	if err == nil {
		return nil
	}
	if err == io.EOF {
		err = nil
	}
	if err1, ok := err.(*net.OpError); ok && err1.Err == syscall.EPIPE {
		// typical close
		err = nil
	}
	return err
}

// proxyFromClient reads from cin, writes to the stream. Should be in a go routine.
// Updates ProxyReadErr and WriteErr
func (s *InOutStream) proxyFromClient(cin io.ReadCloser, errch chan error) {
	_, err := s.ReadFrom(cin)
	// At this point cin either returned an EOF (FIN), or error (RST from remote, or error writing)
	if NoEOF(s.ProxyReadErr) != nil || s.WriteErr != nil {
		// May send RST
		if DebugClose {
			log.Println(s.StreamId, "proxyFromClient RST ", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
		}
		s.Close()
		cin.Close()
	} else {
		if DebugClose {
			log.Println(s.StreamId, "proxyFromClient FIN ", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
		}
		s.CloseWrite()
	}

	errch <- err
}

// Implements net.Addr, can be returned as getRemoteAddr()
// Not ideal: apps will assume IP. Better to return the VIP6.
// Deprecated
type nameAddress string

// name of the network (for example, "tcp", "udp")
func (na nameAddress) Network() string {
	return "mesh"
}
func (na nameAddress) String() string {
	return string(na)
}

// RoundTripStart() must be called for clients to initiate the roundtrip.
// It will not block - client must start writing to the stream, will get back
// the headers with the first response bytes.
//
// Not part of NewGRPCStream because client may set additional headers.
//func (hc *InOutStream) RoundTripStart() {
//	if hc.rtCh != nil || hc.Response != nil {
//		return // already started
//	}
//	hc.rtCh = make(chan error)
//	// Needs to be in a go routine - Write is blocking until bytes are
//	// sent, which happens during roundtrip.
//	go func() {
//		// TODO: RoundTripStart to find the cluster.
//
//		hres, err := hc.Cluster.RoundTrip(hc.Request)
//		if err == nil {
//			hc.Response = hres
//			hc.In = hres.Body
//
//			if hres.StatusCode >= 300 || hres.StatusCode < 200 {
//				err = errors.New(fmt.Sprintf("status code %v",
//					hres.StatusCode))
//			}
//			hc.rbuffer = nio.GetBuffer(0, 16*64*1024)
//		}
//
//		if err != nil {
//			if c, ok := hc.RequestInPipe.(io.Closer); ok {
//				c.Close()
//			}
//		}
//
//		if hc.ErrChan != nil {
//			hc.ErrChan <- err
//		}
//
//		hc.rtCh <- err
//	}()
//}

//func (hc *InOutStream) Close() error {
//	// TODO: send trailers if server !!!
//	//hc.W.Close()
//	if cw, ok := hc.RequestInPipe.(io.Closer); ok {
//		cw.Close()
//	}
//	if cw, ok := hc.In.(io.Closer); ok {
//		return cw.Close()
//	}
//
//	return nil
//}

func (hc *InOutStream) Recv(last bool) (*Buffer, error) {

	if hc.RcvdPackets == 0 {
		err := <-hc.rtCh
		if err != nil {
			return nil, err
		}
	}
	if hc.Response != nil {
		if hc.Response.StatusCode >= 300 || hc.Response.StatusCode < 200 {
			return nil, errors.New(fmt.Sprintf("status code %v",
				hc.Response.StatusCode))
		}
	}

	b := hc.rbuffer

	if hc.RcvdPackets > 0 {
		b.Discard(int(5 + len(hc.lastFrame.Bytes())))
	} else {
		//log.Println("Header: ", hc.Res.Header)
		// Initial headers don't include grpc-status - just 200
		// grpc-encoding - compression

	}

	// TODO: only if an incomplete frame.
	if !b.IsEmpty() {
		b.Compact()
	}

	hc.RcvdPackets++

	var mlen uint32
	head, err := b.Fill(hc.In, 5)
	if err == nil {
		if head[0] != 0 {
			return nil, fmt.Errorf("Invalid frame %d", head[0])
		}
		mlen = binary.BigEndian.Uint32(head[1:])

		_, err = b.Fill(hc.In, int(5+mlen))
	}
	// At this point, Res should be set.

	if err == io.EOF {
		// TODO: extract, see http2-client in grpc
		// grpc-status
		// grpc-message
		// grpc-status-details-bin - base64 proto
		if hc.Response != nil {
			log.Println("Trailer", hc.Response.Trailer)
			hc.Response.Body.Close()
		}
	} else if err != nil {
		return nil, err
	}

	if hc.Response.Trailer.Get("") != "" {

	}

	hc.lastFrame = b.Frame(5, int(5+mlen))
	return hc.lastFrame, err
}

// Return a buffer with reserved front space to be used for appending.
// If using functions like proto.Marshal, b.UpdateForAppend should be called
// with the new []byte. App should not touch the prefix.
func (hc *InOutStream) GetWriteFrame(sz int) *Buffer {
	b := GetBuffer(5+9, sz)
	return b
}

// Framed sending/receiving.
func (hc *InOutStream) Send(b *Buffer) error {

	hc.SentPackets++

	frameLen := b.Len()

	b.SetUnint32BE(b.Start()-4, uint32(frameLen))
	b.SetByte(b.Start()-5, 0)

	_, err := hc.Write(b.Buffer()[b.Start()-5 : b.End()])
	if f, ok := hc.Out.(http.Flusher); ok {
		f.Flush()
	}

	b.Recycle()

	return err
}

