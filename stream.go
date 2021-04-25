package ugate

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)


// Stream is the main abstraction, representing a connection with metadata and additional
// helpers.
//
// The connection is typically:
// - an accepted connection - In/Out are the raw net.Conn
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
// Metrics are maintained.
//
// Implements net.Conn - but does not implement ConnectionState(), so the
// stream can be used with H2 library.
type Stream struct {

	// StreamId is based on a counter, it is the key in the Active table.
	// Streams may also have local ids associated with the transport.
	StreamId int

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
	// Normal process for close is to call CloseWrite, read fully the In and call Close on the stream.
	In io.ReadCloser `json:"-"`

	// Out - send to remote.
	//
	// - an instance of net.Conn or tls.Conn - both implementing CloseWrite for FIN
	// - http.ResponseWriter - for accepted HTTP connections, implements CloseWrite
	// - a Pipe - for dialed HTTP connections, emulating DialContext behavior ( no body sent before connection is
	//   completed)
	// - nil, if the remote side is read only ( GET ) or if the creation of the
	//   stream passed a Reader object which is automatically piped to the Out, for example
	//   when a HTTP request is used.
	Out io.Writer `json:"-"`

	// Request associated with the stream. Will be set if the stream is
	// received over HTTP (real or over another virtual connection),
	// or if the stream is originated locally and sent to a HTTP dest.
	// Deprecated
	Request *http.Request `json:"-"`

	// Set if the connection finished a TLS handshake.
	// A 'dummy' value may be set if a sidecar terminated the connection.
	// Deprecated - moved to MUX
	TLS *tls.ConnectionState `json:"-"`

	// Metadata to send. Stream implements http.ResponseWriter.
	// For streams without metadata - will be ignored.
	// Incoming metadata is set in Request.
	OutHeader http.Header `json:"-"`

	// Header received from the remote.
	// For egress it is the response headers.
	// For ingress it is the request headers.
	InHeader http.Header `json:"-"`

	// Remote mesh ID, if authenticated. Base32(SHA256(PUB)) or Base32(PUB) (for ED)
	// This can be used in DNS names, URLs, etc.
	RemoteID string

	// Original dest - hostname or IP, including port. Parameter of the original Dial from the captured egress stream.
	// May be a mesh IP6, host, etc. If original address was captured by IP, destIP will also be set.
	// Host is extracted from metadata (SOCKS, iptables, etc)
	// Typically a DNS or IP address
	// For example in CONNECT it will be hostname:port or IP:port
	// For HTTP PROXY the path is a full URL.
	Dest string

	// Resolved destination IP. May be nil if SOCKS or forwarding is done. Final Gateway will have it set.
	// If capture is based on IP, it'll be set in all hops.
	// If set, this is the authoritiative destination, DestDNS will be a hint.
	DestAddr *net.TCPAddr

	// Hostname of the destination, based on DNS cache and interception.
	// Used as a key in the 'per host' stats - port not included.
	// Empty if the dest was an IP and DNS interception can't resolve it.
	// Not accurate if DNS interception populated it.
	// TODO: remove or separate the dns-interception based usage.
	DestDNS string

	// Set to the localAddr from the real connection, for http only
	//
	localAddr net.Addr

	// Client type - original capture and all transport hops.
	// SOCKS, CONNECT, PROXY, SOCKSIP, PROXYIP,
	// EPROXY = TCP-over-HTTP in, direct host out
	Type string

	Open time.Time

	// last receive from local (and send to remote)
	LastWrite time.Time

	// last receive from remote (and send to local)
	LastRead time.Time

	// Sent from client to server ( client is initiator of the proxy )
	SentBytes   int
	SentPackets int

	// Received from server to client
	RcvdBytes   int
	RcvdPackets int

	// If set, this is a circuit.
	//NextPath []string

	// Set for circuits - path so far (over H2)
	//PrevPath []string

	// Additional closer, to be called after the proxy function is done and both client and remote closed.
	Closer     func() `json:"-"`

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
	ReadErr  error `json:"-"`
	WriteErr error `json:"-"`

	// Only for 'accepted' streams (server side), in proxy mode: keep track
	// of the client side. The server is driving the proxying.
	ProxyReadErr  error `json:"-"`
	ProxyWriteErr error `json:"-"`


	// Context and cancel funciton for this stream.
	ctx       context.Context `json:"-"`

	// Close will invoke this method if set, and cancel the context.
	ctxCancel context.CancelFunc `json:"-"`

	// Set for accepted connections, with the config associated with the listener.
	Listener *Listener `json:"-"`

	// Optional function to call after dial (proxied streams) or after a stream handling has started for local handlers.
	// Used to send back metadata or finish the handshake.
	//
	// For example in SOCKS it sends back the IP/port of the remote.
	// net.Conn may be a Stream or a regular TCP/TLS connection.
	PostDialHandler func(net.Conn, error) `json:"-"`

	// True if the Stream is originated from local machine, i.e.
	// SOCKS/iptables/TUN capture or dialed from local process
	Egress bool

	// If the stream is multiplexed, this is the Mux.
	MUX *Muxer `json:"-"`

	// ---------------------------------------------------------

}

// Create a new stream.
func NewStream() *Stream {
	return &Stream{
		StreamId: int(atomic.AddUint32(&StreamId, 1)),
		Open:     time.Now(),
	}
}

// Create a new stream from a HTTP request/response.
//
// For accepted requests, http2/server.go newWriterAndRequests populates the request based on the headers.
// Server validates method, path and scheme=http|https. Req.Body is a pipe - similar with what we use for egress.
// Request context is based on stream context, which is a 'with cancel' based on the serverConn baseCtx.
//
func NewStreamRequest(r *http.Request, w http.ResponseWriter, con *Stream) *Stream {
	return &Stream{
		StreamId: int(atomic.AddUint32(&StreamId, 1)),
		Open:     time.Now(),

		Request: r,
		In:      r.Body,
		Out:     w,
		TLS:     r.TLS,
		Dest:    r.Host,
	}
}

func NewStreamRequestOut(r *http.Request, out io.Writer, w *http.Response, con *Stream) *Stream {
	return &Stream{
		StreamId:  int(atomic.AddUint32(&StreamId, 1)),
		Open:      time.Now(),
		OutHeader: w.Header,
		Request:   r,
		In:        w.Body, // Input from remote http
		Out:       out, //
		TLS:       r.TLS,
		Dest:      r.Host,
	}
}

//func (s *Stream) Reset() {
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

func (s *Stream) HTTPRequest() *http.Request {
	if s.Request == nil {
		s.Request = &http.Request{
			Host: s.Dest,
		}
	}
	return s.Request
}

func (s *Stream) HTTPResponse() http.ResponseWriter {
	if rw, ok := s.Out.(http.ResponseWriter); ok {
		return rw
	}
	return nil
}

const ContextKey = "ugate.stream"

// DO NOT IMPLEMENT: H2 will use the ConnectionStater interface to
// detect TLS, and do checks. Would break plain text streams.
// Also auth is more flexibile then mTLS.
//// Used by H2 server to populate TLS in accepted requests.
//// For 'fake' TLS (raw HTTP) it must be populated.
//func (s *Stream) ConnectionState() tls.ConnectionState {
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
func (s *Stream) Context() context.Context {
	//if s.Request != nil {
	//	return s.Request.Context()
	//}
	if s.ctx == nil {
		s.ctx, s.ctxCancel = context.WithCancel(context.Background())
		s.ctx = context.WithValue(s.ctx, ContextKey, s)
	}
	return s.ctx
}

func (s *Stream) Meta() *Stream {
	return s
}

func (s *Stream) Write(b []byte) (n int, err error) {
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
func (s *Stream) Flush() {
	if f, ok := s.Out.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Stream) Read(out []byte) (int, error) {
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
//
func (s *Stream) Close() error {
	if s.Closed {
		return nil
	}
	s.Closed = true
	if s.Closer != nil {
		s.Close()
	}
	if s.ctxCancel != nil {
		s.ctxCancel()
	}
	rw := s.HTTPResponse()
	if rw != nil {
		rw.Header().Set("X-Close", "1")
		if DebugClose {
			log.Println("Close HTTP via trailer", s.StreamId, s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
		}
	}
	if c, ok := s.Out.(io.Closer); ok {
		if DebugClose {
			log.Println(s.StreamId, "Close(out) ", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
		}
		return c.Close()
	} else {
		if DebugClose {
			log.Println(s.StreamId, "Close(in) ", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
		}
		return s.In.Close()
	}
}

func (s *Stream) CloseWrite() error {
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
				log.Println(s.StreamId, "CloseWrite using Out.Close()",  s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
			}
			return c.Close()
		} else {
			rw := s.HTTPResponse()
			if rw != nil {
				// Server side HTTP stream. For client side, FIN can be sent by closing the pipe (or
				// request body). For server, the FIN will be sent when the handler returns - but
				// this only happen after request is completed and body has been read. If server wants
				// to send FIN first - while still reading the body - we are in trouble.

				// That means HTTP2 TCP servers provide no way to send a FIN from server, without
				// having the request fully read.
				if DebugClose {
					log.Println(s.StreamId, "CloseWrite using HTTP trailer ",  s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
				}
				// This works for H2 with the current library.
				rw.Header().Set("X-Close", "0")
				rw.(http.Flusher).Flush()
			} else {
				log.Println("Server out not Closer nor CloseWriter")
			}
		}
	}
	return nil
}

func (s *Stream) SetDeadline(t time.Time) error {
	if cw, ok := s.Out.(net.Conn); ok {
		cw.SetDeadline(t)
	}
	return nil
}

func (s *Stream) SetReadDeadline(t time.Time) error {
	if cw, ok := s.Out.(net.Conn); ok {
		cw.SetReadDeadline(t)
	}
	return nil
}

func (s *Stream) SetWriteDeadline(t time.Time) error {
	if cw, ok := s.Out.(net.Conn); ok {
		cw.SetWriteDeadline(t)
	}
	return nil
}

func (s *Stream) Header() http.Header {
	if rw, ok := s.Out.(http.ResponseWriter); ok {
		return rw.Header()
	}
	if s.OutHeader == nil {
		s.OutHeader = map[string][]string{}
	}
	return s.OutHeader
}

func (s *Stream) WriteHeader(statusCode int) {
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
func (s *Stream) CopyBuffered(dst io.Writer, src io.Reader, srcIsRemote bool) (written int64, err error) {
	buf1 := bufferPoolCopy.Get().([]byte)
	defer bufferPoolCopy.Put(buf1)
	bufCap := cap(buf1)
	buf := buf1[0:bufCap:bufCap]

	// For netstack: src is a gonet.Conn, doesn't implement WriterTo. Dst is a net.TcpConn - and implements ReadFrom.
	// CopyBuffered is the actual implementation of Copy and CopyBuffer.
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

func (s *Stream) LocalAddr() net.Addr {
	if cw, ok := s.Out.(net.Conn); ok {
		return cw.LocalAddr()
	}

	// For HTTP/BTS requests, local addr should be the address from the listener or
	// stream.
	return s.localAddr
}

// RemoteAddr is the client (for accepted) or server (for originated).
// It should be the real IP, extracted from connection or metadata.
// RemoteID returns the authenticated ID.
func (s *Stream) RemoteAddr() net.Addr {
	// non-test Streams are either backed by a net.Conn or a Request
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
	// Dial doesn't set it very well...
	//return tp.SrcAddr
}

// Reads data from cin (the client/dialed con) until EOF or error
// TCP Connections typically implement this, using io.Copy().
func (s *Stream) ReadFrom(cin io.Reader) (n int64, err error) {

	//if wt, ok := cin.(io.WriterTo); ok {
	//	return wt.WriteTo(s.ServerOut)
	//}

	//if _, ok := cin.(*os.File); ok {
	//	if _, ok := b.ServerOut.(*net.TCPConn); ok {
	//		if wt, ok := b.ServerOut.(io.ReaderFrom); ok {
	//			VarzReadFromC.Add(1)
	//			n, err = wt.ReadFrom(cin)
	//			return
	//		}
	//	}
	//}

	// Typical case for accepted connections, TCPConn  implements
	// this efficiently by splicing.
	// TCP conn ReadFrom fallbacks to Copy without recycling the buffer
	if CanSplice(cin, s.Out) {
		if wt, ok := s.Out.(io.ReaderFrom); ok {
			VarzReadFromC.Add(1)
			n, err = wt.ReadFrom(cin)
			s.SentPackets++
			s.SentBytes += int(n)
			return
		}
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
			s.ProxyReadErr = er
			return n, er
		}
		if nr > int(VarzMaxRead.Value()) {
			VarzMaxRead.Set(int64(nr))
		}

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
	}

	return
}

func (b *Stream) PostDial(nc net.Conn, err error) {
	if b.PostDialHandler != nil {
		b.PostDialHandler(nc, err)
	}
}

// If true, will debug or close operations.
// Close is one of the hardest problems, due to FIN/RST multiple interfaces.
const DebugClose = true

// Proxy the accepted connection to a dialed connection.
// Blocking, will wait for both sides to FIN or RST.
func (s *Stream) ProxyTo(nc net.Conn) error {
	errCh := make(chan error, 2)
	go s.proxyFromClient(nc, errCh)
	// Blocking, returns when all data is read from In, or error
	err1 := s.proxyToClient(nc, errCh)


	// Wait for data to be read from nc and sent to Out, or error
	remoteErr := <-errCh
	if remoteErr == nil {
		remoteErr = err1
	}

	// The read part may have returned EOF, or the write may have failed.
	// In the first case close will send FIN, else will send RST
	if DebugClose {
		if strings.HasPrefix(s.Request.RequestURI, "/dm/") {
			log.Println(s.StreamId, "proxyTo H2 ",  s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr, s.Request.RequestURI)
		} else {
			log.Println(s.StreamId, "proxyTo ",  s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
		}
	}
	s.In.Close()
	nc.Close()
	return remoteErr
}

// Read from the Reader, send to the cout client.
// Updates ReadErr and ProxyWriteErr
func (s *Stream) proxyToClient(cout io.WriteCloser, errch chan error) error {
	s.WriteTo(cout) // errors are preserved in stats, 4 kinds possible

	// At this point an error or graceful EOF from our Reader has been received.
	err := s.ProxyWriteErr
	if err == nil {
		err = s.ReadErr
	}

	if NoEOF(err) != nil {
		// Should send RST if unbuffered data (may also be FIN - no way to control)
		if DebugClose {
			log.Println(s.StreamId, "proxyToClient RST",  s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
		}
		cout.Close()
		s.In.Close()
	} else {
		// WriteTo doesn't close the writer ! We need to send a FIN, so remote knows we're done.
		if c, ok := cout.(CloseWriter); ok {
			if DebugClose {
				log.Println(s.StreamId,"proxyToClient EOF", s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
			}
			s.ClientClose = true
			c.CloseWrite()
		} else {
			//if debugClose {
				log.Println(s.StreamId,"proxyToClient EOF, XXX Missing CloseWrite",  s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
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
func (s *Stream) WriteTo(w io.Writer) (n int64, err error) {

	if CanSplice(s.In, w) {
		if wt, ok := w.(io.ReaderFrom); ok {
			VarzReadFrom.Add(1)
			n, err = wt.ReadFrom(s.In)
			s.RcvdPackets++
			s.RcvdBytes += int(n)
			s.LastRead = time.Now()
			return
		}
	}

	buf1 := bufferPoolCopy.Get().([]byte)
	defer bufferPoolCopy.Put(buf1)
	bufCap := cap(buf1)
	buf := buf1[0:bufCap:bufCap]

	for {
		sn, sErr := s.In.Read(buf)
		s.RcvdPackets++
		s.RcvdBytes += sn

		if sn > int(VarzMaxRead.Value()) {
			VarzMaxRead.Set(int64(sn))
		}

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

// proxyFromClient reads from cin, writes to the stream. Should be in a go routine.
// Updates ProxyReadErr and WriteErr
func (s *Stream) proxyFromClient(cin io.ReadCloser, errch chan error) {
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
		//if cc, ok := cin.(CloseReader); ok {
		//	if debugClose {
		//		log.Println("proxyFromClient CloseRead", s.StreamId, s.ReadErr, s.WriteErr, s.ProxyReadErr, s.ProxyWriteErr)
		//	}
		//	cc.CloseRead()
		//}
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

// StreamInfo tracks informations about one stream.
type StreamInfo struct {
	LocalAddr  net.Addr
	RemoteAddr net.Addr

	Meta       http.Header

	RemoteID string

	ALPN     string

	Dest string

	Type string
}

func (str *Stream) StreamInfo() *StreamInfo {
	si := &StreamInfo{
		LocalAddr:  str.LocalAddr(),
		RemoteAddr: str.RemoteAddr(),
		Meta:       str.HTTPRequest().Header,
		Dest:       str.Dest,
		Type:       str.Type,
	}
	if str.TLS != nil {
		si.ALPN = str.TLS.NegotiatedProtocol
	}

	return si
}
