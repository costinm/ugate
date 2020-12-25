package ugate

// Inspired from: github.com/soheilhy/cmux
// Most of the code replaced.

import (
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

// TODO: implemnet ByteReader, ByteWriter, ByteScanner, WriterTo,
// ReadFrom

// TODO: bench with different sizes. Should hold at least few packets
// Used for sniffing first packet and for the io copy from accepted connection.
var bufSize = 32 * 1024

var (
	// createBuffer to get a buffer. io.Copy uses 32k.
	// experimental use shows ~20k max read with Firefox.
	bufferPoolCopy = sync.Pool{New: func() interface{} {
		return make([]byte, 0, 32*1024)
	}}
)

var bufferedConPool = sync.Pool{New: func() interface{} {
	// Should hold a TLS handshake message
	return &RawConn{buf: make([]byte, bufSize)}
}}

func GetConn(in net.Conn) *RawConn {
	br := bufferedConPool.Get().(*RawConn)
	br.raw = in
	br.ResetStats()
	return br
}

// Wraps accepted connections, keeps a buffer and detected metadata.
//
// RawConn is an optimized implementation of io.Reader that behaves like
// ```
// io.MultiReader(bytes.NewReader(buffer.Bytes()), io.TeeReader(source, buffer))
// ```
// without allocating.
//
// Also similar with bufio.Reader, but with recycling and access to buffer,
// metadata, stats and for net.Conn.
// TODO: use net.Buffers ? The net connection likely implements it.
type RawConn struct {
	// Typically a *net.TCPConn, implements ReaderFrom.
	// May also be a TLSConn, etc.
	raw net.Conn

	// if true, anything read will be added to buffer.
	// if false, Read() will consume the buffer from off to end, then use
	// direct Read.
	sniffing bool

	// b has end and capacity, set at creation to the size of the buffer,
	// end(buf) == cap(buf) == 8k
	// using end and off as pointers to data
	buf []byte

	// read so far from buffer. Unread data in off:last
	off int

	// number of bytes in buffer.
	end int

	// If an error happened while sniffing
	lastErr error

	// Optional function to call after dial. Used to send metadata
	// back to the protocol ( for example SOCKS)
	postDial func(net.Conn, error)

	Stats Stats
}

func (b *RawConn) PostDial(nc net.Conn, err error) {
	if b.postDial != nil {
		b.postDial(nc, err)
	}
}

func (b *RawConn) Meta() *Stats {
	return &b.Stats
}

func (b *RawConn) LocalAddr() net.Addr {
	return b.raw.LocalAddr()
}

func (b *RawConn) RemoteAddr() net.Addr {
	return b.raw.RemoteAddr()
}

func (b *RawConn) SetDeadline(t time.Time) error {
	return b.rwConn().SetDeadline(t)
}

func (b *RawConn) SetReadDeadline(t time.Time) error {
	return b.rwConn().SetReadDeadline(t)
}

func (b *RawConn) SetWriteDeadline(t time.Time) error {
	return b.rwConn().SetWriteDeadline(t)
}

func (b *RawConn) Close() error {
	// Remove it from the tracker
	if b.Stats.ContextCancel != nil {
		b.Stats.ContextCancel()
	}
	return b.raw.Close()
}

func (b *RawConn) rwConn() net.Conn {
	return b.raw
}

func (b *RawConn) Write(p []byte) (int, error) {
	n, err := b.rwConn().Write(p)
	if err != nil {
		b.Stats.WriteErr = err
		return n, err
	}
	b.Stats.WritePackets++
	b.Stats.WriteBytes+= n
	b.Stats.LastWrite = time.Now()
	return n, err
}
func (b *RawConn) empty() bool {
	return b.off >= b.end
}

func (b *RawConn) Len() int {
	//
	return b.end - b.off
}

// Return the unread portion of the buffer
func (b *RawConn) Bytes() []byte {
	return b.buf[b.off:b.end]
}

func (b *RawConn) ReadByte() (byte, error) {
	if b.empty() {
		err := b.Fill()
		if err != nil {
			return 0, err
		}
	}
	r := b.buf[b.off]
	b.off++
	return r, nil
}

// Fill the buffer by doing one Read() from the underlying reader.
// Calls to Read() will use the buffer.
func (b *RawConn) Fill() (error) {
	if b.empty() && !b.sniffing {
		b.off = 0
		b.end = 0
	}
	n, err := b.rwConn().Read(b.buf[b.end:])
	b.end += n
	if err != nil {
		return err
	}
	return nil
}

func (b *RawConn) Buffer() []byte {
	return b.buf[b.off:b.end]
}

func (b *RawConn) Read(p []byte) (int, error) {
	if b.end > b.off {
		// If we have already read something from the buffer before, we return the
		// same data and the last error if any. We need to immediately return,
		// otherwise we may block for ever, if we try to be smart and call
		// source.Read() seeking a little bit of more data.
		bn := copy(p, b.buf[b.off:b.end])
		b.off += bn
		if !b.sniffing && b.end <= b.off {
			// buffer has been consummed, not in sniff mode
			b.off = 0
			b.end = 0
		}
		return bn, b.lastErr
	}
	// AcceptedCon is reused, keep the buffer around

	// If there is nothing more to return in the sniffed buffer, read from the
	// source.
	sn, sErr := b.rwConn().Read(p)
	if sn > 0 && b.sniffing {
		b.lastErr = sErr
		if len(b.buf) < b.end+ sn {
			return sn, errors.New("Short buffer")
		}
		copy(b.buf[b.end:], p[:sn])
		b.end += sn
	}
	b.Stats.ReadPackets++
	b.Stats.ReadBytes+= sn
	if sErr != nil {
		b.Stats.ReadErr = sErr
	}
	b.Stats.LastRead = time.Now()
	return sn, sErr
}

// Reset will set the bufferRead to off, so Read() will start from there (ignoring
// bytes from header).
// Sniffing mode is disabled.
func (b *RawConn) Reset(off int) {
	b.sniffing = false
	b.off = off
}

func (b *RawConn) Clean() {
	b.sniffing = false
	b.off = 0
	b.end = 0
}

func (b *RawConn) Sniff() {
	b.sniffing = true
	b.off = 0
	b.end = 0
}

func (b *RawConn) ResetStats() {
	b.Stats.Reset()
}

// Proxy the accepted connection to a dialed connection.
// Blocking, will wait for both sides to FIN or RST.
func (b *RawConn) Proxy(cl net.Conn) error {
	errCh := make(chan error, 2)
	go b.ProxyFromClient(cl, errCh)
	return b.ProxyToClient(cl, errCh)
}

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

// WriteTo implements the interface, using the read buffer.
func (b *RawConn) WriteTo(w io.Writer) (n int64, err error) {
	// Finish up the buffer first
	if !b.empty() {
		bn, err := w.Write(b.buf[b.off:b.end])
		if err != nil {
			//"Write must return non-nil if it doesn't write the full buffer"
			b.Stats.ProxyWriteErr = err
			return int64(bn), err
		}
		b.off += bn
		n += int64(bn)
	}

	// but the dialed connection might, so we can splice
	if CanSplice(b.rwConn(), w) {
		if wt, ok := w.(io.ReaderFrom); ok {
			VarzReadFrom.Add(1)
			n, err = wt.ReadFrom(b.rwConn())
			b.Stats.ReadPackets++
			b.Stats.ReadBytes += int(n)
			return
		}
	}

	for {
		sn, sErr := b.rwConn().Read(b.buf)
		b.Stats.ReadPackets++
		b.Stats.ReadBytes += sn

		if sn > int(VarzMaxRead.Value()) {
			VarzMaxRead.Set(int64(sn))
		}

		if sn > 0 {
			wn, wErr := w.Write(b.buf[0:sn])
			n += int64(wn)
			if wErr != nil {
				b.Stats.ProxyWriteErr = wErr
				return n, wErr
			}
		}
		// May return err but still have few bytes
		if sErr != nil {
			sErr = eof(sErr)
			b.Stats.ReadErr = sErr
			return n, sErr
		}
	}
}

// Used for Proxy to send data to the dialed connection and coordinante
// the finish. This is foreground.
func (b *RawConn) ProxyToClient(cin io.Writer, errch chan error) error {
	b.WriteTo(cin) // errors are preserved in stats, 4 kinds possible

	// WriteTo doesn't close the writer !
	if c, ok := cin.(CloseWriter); ok {
		c.CloseWrite()
	}

	remoteErr := <- errch

	// The read part may have returned EOF, or the write may have failed.
	// In the first case close will send FIN, else will send RST
	b.rwConn().Close()

	if c, ok := cin.(io.Closer); ok {
		c.Close()
	}

	return remoteErr
}

// Reads data from cin (the client/dialed con) until EOF or error
// TCP Connections typically implement this, using io.Copy().
func (b *RawConn) ReadFrom(cin io.Reader) (n int64, err error) {
	// Typical case - accepted connections are TCPConn and implement
	// this efficiently
	// However ReadFrom fallbacks to Copy without recycling the buffer
	//
	if _, ok := cin.(*os.File); ok {
		if _, ok := b.rwConn().(*net.TCPConn); ok {
			if wt, ok := b.rwConn().(io.ReaderFrom); ok {
				VarzReadFromC.Add(1)
				n, err = wt.ReadFrom(cin)
				return
			}
		}
	}

	if CanSplice(cin, b.rwConn()) {
		if wt, ok := b.rwConn().(io.ReaderFrom); ok {
			VarzReadFromC.Add(1)
			n, err = wt.ReadFrom(cin)
			b.Stats.WritePackets++
			b.Stats.WriteBytes += int(n)
			return
		}
	}

	//if _, ok := cin.(*net.UnixConn); ok {
	//	if wt, ok := b.rwConn().(io.ReaderFrom); ok {
	//		return wt.ReadFrom(cin)
	//	}
	//}
	//if wt, ok := cin.(io.WriterTo); ok {
	//	return wt.WriteTo(b.rwConn())
	//}

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

		nw, err := b.rwConn().Write(buf[0:nr])
		n += int64(nw)
		b.Stats.WriteBytes += nw
		b.Stats.WritePackets++
		if err != nil {
			return n, err
		}
	}

	return
}

// ProxyFromClient writes to the net.Conn. Should be in a go routine.
func (b *RawConn) ProxyFromClient(cin io.Reader, errch chan error)  {
	_, err := b.ReadFrom(cin)

	// At this point either cin returned FIN or RST

	if cw, ok := b.rwConn().(CloseWriter); ok {
		cw.CloseWrite()
	}
	errch <- err
}

func eof(err error) error {
	if err == io.EOF {
		err = nil
	}
	if err1, ok := err.(*net.OpError); ok && err1.Err == syscall.EPIPE {
		// typical close
		err = nil
	}
	return err
}
