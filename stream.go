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

// Connection with metadata
// Common to TCP and UDP proxies
// - Represents an outgoing connection to a remote site, with stats and metadata.
// - Also represents an incoming connection from a remote
//
// Implements net.Conn
//
// Example:
// - raw TCP connection
// - SOCKS - extracted dest host:port or IP:port
// - IPTables - extracted original DST IP:port
// - SNI - extracted 'Server Name' - port based on the listener port
// - TLS - peer certificates, SNI, ALPN
// -
type Stream struct {
	// Counter
	// Key in the Active table.
	StreamId int

	// remote In - data from remote app to local.
	// May be an instance:
	// - net.Conn - for outbound TCP connections
	// - a res.Body for http-over-HTTP client. Note that remoteOut will be null in this case.
	// - a TCPConnection for socks
	// - for ssh -
	//
	// Closing the 'input' without fully reading will result in RST.
	ServerIn io.ReadCloser `json:"-"`

	// remoteOut - stream sending to the server.
	// will be nil for http or cases where the API uses Read() and has its own local->remote proxy logic.
	//
	// Normally an instance of net.Conn, create directly to app or to another node.
	// Not WriterCloser because it can be an http.ResponseWriter, which implements
	// CloseWrite instead.
	ServerOut io.Writer `json:"-"`

	// Associated virtual http request.
	Request *http.Request `json:"-"`

	// Set if the connection finished a TLS handshake.
	// A 'dummy' value may be set if a sidecar terminated the connection.
	TLS *tls.ConnectionState `json:"-"`

	// Metadata to send on response. Stream implements http.ResponseWriter.
	// For streams without metadata - will be ignored.
	ResponseHeader http.Header

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
	// Used as a key in the 'per host' stats.
	DestDNS string

	// Set to the localAddr from the connection, for HTTP requests only
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
	NextPath []string

	// Set for circuits - path so far (over H2)
	PrevPath []string

	// Additional closer, to be called after the proxy function is done and both client and remote closed.
	Closer     func() `json:"-"`
	ReadCloser func() `json:"-"`

	// Set if CloseWrite() was called, which should result in a FIN sent.
	// This should happen if a EOF was received when proxying.
	ServerClose bool `json:"-"`

	// Set if Close() was called.
	Closed bool `json:"-"`

	// Errors associated with this stream, read from or write to.
	ReadErr  error `json:"-"`
	WriteErr error `json:"-"`

	ctx       context.Context `json:"-"`
	ctxCancel context.CancelFunc `json:"-"`

	// Set for accepted connections, with the config associated with the listener.
	Listener *Listener `json:"-"`

	// Optional function to call after dial. Used to send metadata
	// back to the protocol ( for example SOCKS)
	postDial func(net.Conn, error) `json:"-"`

	// True if the Stream is originated from local machine, i.e.
	// SOCKS/iptables/TUN capture or dialed from local process
	Egress bool

	// ---------------------------------------------------------

	// Only for 'accepted' streams (server side), in proxy mode: keep track
	// of the client side. The server is driving the proxying.
	ProxyReadErr  error `json:"-"`
	ProxyWriteErr error `json:"-"`

	// Set if the client has sent the FIN, and gateway sent the FIN to server
	ClientClose bool `json:"-"`
}

// Create a new stream.
func NewStream() *Stream {
	return &Stream{
		StreamId: int(atomic.AddUint32(&streamIds, 1)),
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
		StreamId: int(atomic.AddUint32(&streamIds, 1)),
		Open:     time.Now(),

		Request:   r,
		ServerIn:  r.Body,
		ServerOut: w,
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
	if rw, ok := s.ServerOut.(http.ResponseWriter); ok {
		return rw
	}
	return nil
}

const ContextKey = "ugate.stream"

// Used by H2 server to populate TLS in accepted requests.
// For 'fake' TLS (raw HTTP) it must be populated.
func (s *Stream) ConnectionState() tls.ConnectionState {
	if s.TLS == nil {
		return tls.ConnectionState{Version: tls.VersionTLS12}
	}
	return *s.TLS
}

func (s *Stream) Context() context.Context {
	if s.Request != nil {
		return s.Request.Context()
	}
	if s.ctx == nil {
		s.ctx, s.ctxCancel = context.WithCancel(context.Background())
		s.ctx = context.WithValue(s.ctx, ContextKey, s)
	}
	return s.ctx
}

func (s *Stream) Meta() *Stream {
	return s
}

func (s *Stream) RemoteID() string {
	if s.TLS == nil {
		return ""
	}
	pk, err := PubKeyFromCertChain(s.TLS.PeerCertificates)
	if err != nil {
		return ""
	}

	return IDFromPublicKey(pk)
}

func (s *Stream) Write(b []byte) (n int, err error) {
	n, err = s.ServerOut.Write(b)
	if err != nil {
		s.WriteErr = err
		return n, err
	}
	s.SentBytes += n
	s.SentPackets++
	s.LastWrite = time.Now()
	if f, ok := s.ServerOut.(http.Flusher); ok {
		f.Flush()
	}

	return
}
func (s *Stream) Flush() {
	if f, ok := s.ServerOut.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Stream) Read(out []byte) (int, error) {
	n, err := s.ServerIn.Read(out)
	s.RcvdBytes += n
	s.RcvdPackets++
	s.LastRead = time.Now()
	err = eof(err)
	if err != nil {
		log.Println("XXXXX CLOSE ", err)
		if s.ReadCloser != nil {
			s.ReadCloser()
			s.ReadCloser = nil
		}
		s.ReadErr = err
	}
	return n, err
}

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
	if c, ok := s.ServerOut.(io.Closer); ok {
		return c.Close()
	} else {
		return s.ServerIn.Close()
	}
}

func (s *Stream) CloseWrite() error {
	if s.ServerClose {
		log.Println("Double CloseWrite")
		return nil
	}
	s.ServerClose = true

	if cw, ok := s.ServerOut.(CloseWriter); ok {
		return cw.CloseWrite()
	}

	if c, ok := s.ServerOut.(io.Closer); ok {
		log.Println("ServerOut is not CloseWriter - closing full connection")
		return c.Close()
	}
	return nil
}

func (s *Stream) SetDeadline(t time.Time) error {
	if cw, ok := s.ServerOut.(net.Conn); ok {
		cw.SetDeadline(t)
	}
	return nil
}

func (s *Stream) SetReadDeadline(t time.Time) error {
	if cw, ok := s.ServerOut.(net.Conn); ok {
		cw.SetReadDeadline(t)
	}
	return nil
}

func (s *Stream) SetWriteDeadline(t time.Time) error {
	if cw, ok := s.ServerOut.(net.Conn); ok {
		cw.SetWriteDeadline(t)
	}
	return nil
}

func (s *Stream) Header() http.Header {
	if rw, ok := s.ServerOut.(http.ResponseWriter); ok {
		return rw.Header()
	}
	if s.ResponseHeader == nil {
		s.ResponseHeader = map[string][]string{}
	}
	return s.ResponseHeader
}

func (s *Stream) WriteHeader(statusCode int) {
	if rw, ok := s.ServerOut.(http.ResponseWriter); ok {
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
				return written, nil
			}
			return written, err
		}
		if nr == 0 {
			// shouldn't happen unless err == io.EOF
			return written, nil
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
	if cw, ok := s.ServerOut.(net.Conn); ok {
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
	if cw, ok := s.ServerOut.(net.Conn); ok {
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
	log.Println("RemoteAddr fallback", s)
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
	if CanSplice(cin, s.ServerOut) {
		if wt, ok := s.ServerOut.(io.ReaderFrom); ok {
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
			er = eof(err)
			return n, er
		}
		if nr > int(VarzMaxRead.Value()) {
			VarzMaxRead.Set(int64(nr))
		}

		nw, err := s.ServerOut.Write(buf[0:nr])
		n += int64(nw)
		s.SentBytes += nw
		s.SentPackets++
		if f, ok := s.ServerOut.(http.Flusher); ok {
			f.Flush()
		}

		if err != nil {
			return n, err
		}
	}

	return
}

func (b *Stream) PostDial(nc net.Conn, err error) {
	if b.postDial != nil {
		b.postDial(nc, err)
	}
}

// Proxy the accepted connection to a dialed connection.
// Blocking, will wait for both sides to FIN or RST.
func (s *Stream) ProxyTo(nc net.Conn) error {
	errCh := make(chan error, 2)
	go s.proxyFromClient(nc, errCh)
	return s.proxyToClient(nc, errCh)
}

// Read from the Reader, send to the client.
// Should be used on accepted (server) connections.
func (s *Stream) proxyToClient(cout io.WriteCloser, errch chan error) error {
	s.WriteTo(cout) // errors are preserved in stats, 4 kinds possible

	// At this point an error or graceful EOF from Reader has been received.
	// TODO: if error do a Close instead of CloseWrite (so EOF is not sent if
	// possible ).
	if s.ProxyWriteErr != nil || s.ReadErr != nil {
		cout.Close()
	} else {
		// WriteTo doesn't close the writer !
		if c, ok := cout.(CloseWriter); ok {
			s.ClientClose = true
			c.CloseWrite()
		} else {
			log.Println("Missing CloseWrite ", cout)
			cout.Close()
		}
	}

	// EOF was received already for normal close.
	// If a write error happened - we want to close it to force a RST.
	if cc, ok := s.ServerIn.(CloseReader); ok {
		cc.CloseRead()
	} else {
		s.ServerIn.Close()
	}

	remoteErr := <-errch

	// The read part may have returned EOF, or the write may have failed.
	// In the first case close will send FIN, else will send RST
	if c, ok := s.ServerOut.(io.Closer); ok {
		c.Close()
	}
	if c, ok := cout.(io.Closer); ok {
		c.Close()
	}

	return remoteErr
}

// WriteTo implements the interface, using the read buffer.
func (s *Stream) WriteTo(w io.Writer) (n int64, err error) {

	if CanSplice(s.ServerIn, w) {
		if wt, ok := w.(io.ReaderFrom); ok {
			VarzReadFrom.Add(1)
			n, err = wt.ReadFrom(s.ServerIn)
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
		sn, sErr := s.ServerIn.Read(buf)
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
		sErr = eof(sErr)
		// May return err but still have few bytes
		if sErr != nil {
			s.ReadErr = sErr
			return n, sErr
		}
	}
}

// proxyFromClient writes to the net.Conn. Should be in a go routine.
func (s *Stream) proxyFromClient(cin io.ReadCloser, errch chan error) {
	_, err := s.ReadFrom(cin)

	// At this point either cin returned FIN or RST
	if s.ProxyReadErr != nil || s.WriteErr != nil {
		s.Close()
	} else {
		s.CloseWrite()
	}

	if cc, ok := cin.(CloseReader); ok {
		cc.CloseRead()
	} else {
		cin.Close()
	}

	errch <- err
}

type nameAddress string

// name of the network (for example, "tcp", "udp")
func (na nameAddress) Network() string {
	return "mesh"
}
func (na nameAddress) String() string {
	return string(na)
}
