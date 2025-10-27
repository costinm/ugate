package nio2

import (
	"context"
	"crypto/tls"
	"errors"
	"expvar"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CloseWriter is one of possible interfaces implemented by RequestInPipe to send a FIN, without closing
// the input. Some writers only do this when Close is called.
type CloseWriter interface {
	CloseWrite() error
}

// ContextGetter allows getting a Context associated with a stream or
// request or other high-level object. Based on http.Request
type ContextGetter interface {
	Context() context.Context
}

// Stream interface extends net.Conn with a context and metadata.
type Stream interface {
	net.Conn
	//context.Context
	StreamMeta
	ContextGetter
}

// Streams tracks active streams. Some streams are long-lived and used to mux other streams.
type Streams struct {
	active sync.Map // streamID -> Stream
}

func (ug *Streams) OnStream(str Stream) {
}

// Called at the end of the connection handling. After this point
// nothing should use or refer to the connection, both proxy directions
// should already be closed for write or fully closed.
func (ug *Streams) OnStreamDone(str Stream) {
	if r := recover(); r != nil {
		debug.PrintStack()

		// find out exactly what the error was and set err
		var err error

		switch x := r.(type) {
		case string:
			err = errors.New(x)
		case error:
			err = x
		default:
			err = errors.New("Unknown panic")
		}
		slog.WarnContext(str.Context(), "panic", "err", err)
	}
}

type StreamMeta interface {
	State() *StreamState

	// Also part of ResponseWriter - it is the response header.
	Header() http.Header

	RequestHeader() http.Header

	//TransportConn() net.Conn

	TLSConnectionState() *tls.ConnectionState
}

var (
	VarzSErrRead  = expvar.NewInt("ugate_srv_err_read_total")
	VarzSErrWrite = expvar.NewInt("ugate_srv_err_write_total")
	VarzCErrRead  = expvar.NewInt("ugate_client_err_read_total")
	VarzCErrWrite = expvar.NewInt("ugate_client_err_write_total")

	VarzMaxRead = expvar.NewInt("ugate_max_read_bytes")

	// Managed by 'NewTCPProxy' - before dial.
	TcpConTotal = expvar.NewInt("gate_tcp_total")

	// Managed by updateStatsOnClose - including error cases.
	TcpConActive = expvar.NewInt("gate_tcp_active")
)

// TODO: use net.Dialer.DialContext(ctx context.Context, network, address string) (InOutStream, error)
// Dialer also uses nettrace in context, calls resolver,
// can do parallel or serial calls. Supports TCP, UDP, Unix, IP

// nettrace: internal, uses TraceKey in context used for httptrace,
// but not exposed. Net has hooks into it.
// RoundTripStart, lookup keep track of DNSStart, DNSDone, ConnectStart, ConnectDone

// httptrace:
// WithClientTrace(ctx, trace) Context
// ContextClientTrace(ctx) -> *ClientTrace

// Stats holds telemetry for a stream or peer.
type Stats struct {
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
}

// StreamState provides metadata about a stream.
//
// It includes errors, stats, other metadata.
// The Stream interface wraps a net.Conn with context and state.
type StreamState struct {
	// InOutStream MuxID - odd for streams initiated from server (push and reverse)
	// Unique withing a mux connection.
	//MuxID uint32

	// It is the key in the Active table.
	// Streams may also have local ids associated with the transport.
	StreamId string

	// WritErr indicates that Write failed - timeout or a RST closing the stream.
	WriteErr error `json:"-"`
	// ReadErr, if not nil, indicates that Read() failed - connection was closed with RST
	// or timedout instead of FIN
	ReadErr error `json:"-"`

	Stats

	// Original or infered destination.
	Dest string
}

// TODO: benchmark different sizes.
var Debug = false
var DebugRW = false

// ReaderCopier copies from In to Out, keeping track of copied bytes and errors.
type ReaderCopier struct {
	// Number of bytes copied.
	Written int64
	MaxRead int
	ReadCnt int

	// First error - may be on reading from In (InError=true) or writing to Out.
	Err error

	InError bool

	In io.Reader

	// For tunneled connections, this can be a tls.Writer. Close will write an TOS close.
	Out io.Writer

	// An ID of the copier, for debug purpose.
	ID string

	// Set if out doesn't implement Flusher and a separate function is needed.
	// Example: tunneled mTLS over http, Out is a tls.Conn which writes to a http Body.
	Flusher http.Flusher
}

func (rc *ReaderCopier) Close() {
	if c, ok := rc.In.(io.Closer); ok {
		c.Close()
	}
	if c, ok := rc.Out.(io.Closer); ok {
		c.Close()
	}

}

// Verify if in and out can be spliced. Used by proxy code to determine best
// method to copy.
//
// Tcp connections implement ReadFrom, not WriteTo
// ReadFrom is only spliced in few cases
func CanSplice(in io.Reader, out io.Writer) bool {
	if _, ok := in.(*net.TCPConn); ok {
		if _, ok := out.(*net.TCPConn); ok {
			return true
		}
	}
	return false
}

// Old style buffer pool

var (
	// createBuffer to get a buffer. io.Copy uses 32k.
	bufferPoolCopy = sync.Pool{New: func() interface{} {
		return make([]byte, 16*64*1024) // 1M
	}}
)

// Varz interface.
// Varz is a wrapper for atomic operation, with a json http interface.
// MetricReader, OTel etc can directly use them.
var (
// Number of copy operations using slice.
// Varz/ReadFromC = expvar.NewInt("io_copy_slice_total2")
)

var StreamId uint32

// Copy will copy src to dst, using a pooled intermediary buffer.
//
// Blocking, returns when src returned an error or EOF/graceful close.
//
// May also return with error if src or dst return errors.
//
// Copy may be called in a go routine, for one of the streams in the
// connection - the stats and error are returned on a channel.
func (s *ReaderCopier) Copy(ch chan int, close bool) {
	if ch != nil {
		defer func() {
			ch <- int(0)
		}()
	}

	if CanSplice(s.In, s.Out) {
		n, err := s.Out.(io.ReaderFrom).ReadFrom(s.In)
		s.Written += n
		if err != nil {
			s.rstWriter(err)
			s.Err = err
		}
		//VarzReadFromC.Add(1)
		return
	}

	buf1 := bufferPoolCopy.Get().([]byte)
	defer bufferPoolCopy.Put(buf1)
	bufCap := cap(buf1)
	buf := buf1[0:bufCap:bufCap]

	//st := ReaderCopier{}

	// For netstack: src is a gonet.ReaderCopier, doesn't implement WriterTo. Dst is a net.TcpConn - and implements ReadFrom.
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
	if s.ID == "" {
		s.ID = strconv.Itoa(int(atomic.AddUint32(&StreamId, 1)))
	}
	if Debug {
		log.Println(s.ID, "startCopy()")
	}
	for {
		if srcc, ok := s.In.(net.Conn); ok {
			srcc.SetReadDeadline(time.Now().Add(15 * time.Minute))
		}
		nr, er := s.In.Read(buf)
		if DebugRW && nr < 1024 {
			log.Println(s.ID, "read()", nr, er)
		}
		if nr > s.MaxRead {
			s.MaxRead = nr
		}

		// Even if we have an error, send the bytes we've read.
		if nr > 0 { // before dealing with the read error
			s.ReadCnt++
			// If RequestInPipe is a ResponseWriter, bad things may happen.
			// There is no deadline - the buffer is put on a queue, and then there is a wait on a ch.
			// The ch is signaled when the frame is sent - if window update has been received.
			// We could try to add a deadline - or directly expose the flow control.
			// See server.go writeDataFromHandler.

			// Write will never return hanging the handler if the client doesn't read. No way to interupt.
			// This may happen if the client is done but didn't close the connection or request, it
			// may still be sending.

			// DoneServing is checked - so it is possible to do this in background, but only works for proxy.

			nw, ew := s.Out.Write(buf[0:nr])
			if DebugRW && nw < 1024 {
				log.Println(s.ID, "write()", nw, ew)
			}
			if nw > 0 {
				s.Written += int64(nw)
			}
			if f, ok := s.Out.(http.Flusher); ok {
				f.Flush()
			}
			if nr != nw && ew == nil { // Should not happen
				ew = io.ErrShortWrite
				if Debug {
					log.Println(s.ID, "write error - short write", s.Err)
				}
			}
			if ew != nil {
				s.Err = ew
				if close {
					s.rstWriter(ew)
				}
				if Debug {
					log.Println(s.ID, "write error rst writer, close in", close, s.Err)
				}
				return
			}
		}

		// Handle Read errors - EOF or real error
		if er != nil {
			if strings.Contains(er.Error(), "NetworkIdleTimeout") {
				er = io.EOF
			}
			if er == io.EOF {
				if Debug {
					log.Println(s.ID, "EOF received, closing writer", close)
				}
				if close {
					// read is already closed - we need to close out
					// TODO: if err is not nil, we should send RST not FIN
					closeWriter(s.Out)
					// close in as well - won't receive more data.
					// However: in many cases this causes the entire net.Conn to close
					//if c, ok := s.In.(io.Closer); ok {
					//	c.Close()
					//}
				}
			} else {
				s.Err = er
				s.InError = true
				if Debug {
					log.Println(s.ID, "readError()", s.Err)
				}
				if close {
					// read is already closed - we need to close out
					// TODO: if err is not nil, we should send RST not FIN
					s.rstWriter(er)
				}
			}

			if Debug {
				log.Println(s.ID, "read DONE", close, s.Err)
			}
			return
		}
	}
}

func (s *ReaderCopier) rstWriter(err error) error {
	if c, ok := s.In.(io.Closer); ok {
		// Otherwise it keeps getting data - this should send a RST
		// TODO: should have a method that also allows errr to be set.
		c.Close()
	}
	dst := s.Out
	if c, ok := dst.(io.Closer); ok {
		return c.Close()
	}
	if c, ok := s.In.(io.Closer); ok {
		// Otherwise it keeps getting data - this should send a RST
		// TODO: should have a method that also allows errr to be set.
		c.Close()
	}
	if rw, ok := dst.(http.ResponseWriter); ok {
		// Server side HTTP stream. For client side, FIN can be sent by closing the pipe (or
		// request body). For server, the FIN will be sent when the handler returns - but
		// this only happen after request is completed and body has been read. If server wants
		// to send FIN first - while still reading the body - we are in trouble.

		// That means HTTP2 TCP servers provide no way to send a FIN from server, without
		// having the request fully read.

		// This works for H2 with the current library - but very tricky, if not set as trailer.
		rw.Header().Set("X-Close", "0")
		rw.(http.Flusher).Flush()
		return nil
	}
	log.Println("Server out not Closer nor CloseWriter nor ResponseWriter", dst)
	return nil
}

func closeWriter(dst io.Writer) error {
	if cw, ok := dst.(CloseWriter); ok {
		return cw.CloseWrite()
	}
	if c, ok := dst.(io.Closer); ok {
		return c.Close()
	}
	if rw, ok := dst.(http.ResponseWriter); ok {
		// Server side HTTP stream. For client side, FIN can be sent by closing the pipe (or
		// request body). For server, the FIN will be sent when the handler returns - but
		// this only happen after request is completed and body has been read. If server wants
		// to send FIN first - while still reading the body - we are in trouble.

		// That means HTTP2 TCP servers provide no way to send a FIN from server, without
		// having the request fully read.

		// This works for H2 with the current library - but very tricky, if not set as trailer.
		rw.Header().Set("X-Close", "0")
		rw.(http.Flusher).Flush()
		return nil
	}
	log.Println("Server out not Closer nor CloseWriter nor ResponseWriter", dst)
	return nil
}

//type tlsHandshakeTimeoutError struct{}
//
//func (tlsHandshakeTimeoutError) Timeout() bool   { return true }
//func (tlsHandshakeTimeoutError) Temporary() bool { return true }
//func (tlsHandshakeTimeoutError) Error() string   { return "net/http: TLS handshake timeout" }

// HandshakeTimeout wraps tlsConn.Handshake with a timeout, to prevent hanging connection.
//func HandshakeTimeout(tlsConn *tls.Conn, d time.Duration, plainConn net.Conn) error {
//ctx, cf := context.WithTimeout(context.Background(), 3 * time.Second)
//defer cf()
//
//err := tlsConn.HandshakeContext(ctx)
//errc := make(chan error, 2)
//var timer *time.Timer // for canceling TLS handshake
//if d == 0 {
//	d = 3 * time.Second
//}
//timer = time.AfterFunc(d, func() {
//	errc <- tlsHandshakeTimeoutError{}
//})
//go func() {
//	err := tlsConn.Handshake()
//	if timer != nil {
//		timer.Stop()
//	}
//	errc <- err
//}()
//if err := <-errc; err != nil {
//	if plainConn != nil {
//		plainConn.Close()
//	} else {
//		tlsConn.Close()
//	}
//	return err
//}
//return nil
//}

// ErrDeadlineExceeded is returned for an expired deadline.
// This is exported by the os package as os.ErrDeadlineExceeded.
var ErrDeadlineExceeded error = &DeadlineExceededError{}

// DeadlineExceededError is returned for an expired deadline.
type DeadlineExceededError struct{}

// Implement the net.Error interface.
// The string is "i/o timeout" because that is what was returned
// by earlier Go versions. Changing it may break programs that
// match on error strings.
func (e *DeadlineExceededError) Error() string   { return "i/o timeout" }
func (e *DeadlineExceededError) Timeout() bool   { return true }
func (e *DeadlineExceededError) Temporary() bool { return true }

// Basic net.Conn stream.
