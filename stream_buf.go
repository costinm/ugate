package ugate

// A net.Conn with metadata and associated resetable buffer, can be used to sniff
// incoming data, intended for handling a raw TCP accepted connection.
//
// The metadata is mapped to a http.Request plus additional stats. Implement
// http.ResponseWriter.
//
// In addition supports efficient 'splice' when proxying to a TCP connection.
//
// Inspired from: github.com/soheilhy/cmux
// Most of the code replaced.

import (
	"errors"
	"io"
	"net"
	"sync"
	"syscall"
	"time"
)

// TODO: implemnet ByteReader, ByteWriter, ByteScanner

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

var BufferedConPool = sync.Pool{New: func() interface{} {
	// Should hold a TLS handshake message
	return &BufferedStream{
		Buf:    make([]byte, bufSize),
		IpBuf:  make([]byte, net.IPv6len),
		Stream: &Stream{},
	}
}}

// GetBufferedStream should be used to get a recycled stream.
func GetBufferedStream(out io.Writer, in io.ReadCloser) *BufferedStream {
	br := BufferedConPool.Get().(*BufferedStream)
	br.resetStats()
	br.Out = out
	br.In = in
	br.sniffing = false
	br.off = 0
	br.End = 0
	br.recycled = false
	return br
}

// BufferedStream is an optimized implementation of io.Reader that behaves like
// ```
// io.MultiReader(bytes.NewReader(buffer.Bytes()), io.TeeReader(source, buffer))
// ```
// without allocating.
//
// Also similar with bufio.Reader, but with recycling and access to buffer,
// metadata, stats and for net.Conn.
// TODO: use net.Buffers ? The net connection likely implements it.
type BufferedStream struct {
	*Stream

	// Typically a *net.TCPConn, implements ReaderFrom.
	// May also be a TLSConn, etc.
	//ServerOut net.Conn

	// if true, anything read will be added to buffer.
	// if false, Read() will consume the buffer from off to End, then use
	// direct Read.
	sniffing bool

	// b has end and capacity, set at creation to the size of the buffer,
	// end(Buf) == cap(Buf) == 8k
	// using end and off as pointers to data
	Buf []byte

	// read so far from buffer. Unread data in off:End (TODO: use len instead of End)
	off int

	// last bytes with data in the buffer.
	End int

	// If an error happened while sniffing
	lastErr error

	IpBuf []byte

	// True if the buffer has been recycled. Will panic if any operation is done.
	recycled bool

	// TODO: if a Read or Write are in progress.
	active bool
}

func (b *BufferedStream) Recycle() {
	b.recycled = true
	BufferedConPool.Put(b)
}

func (b *BufferedStream) empty() bool {
	return b.off >= b.End
}

func (b *BufferedStream) Len() int {
	//
	return b.End - b.off
}

// Return the unread portion of the buffer
func (b *BufferedStream) Bytes() []byte {
	return b.Buf[b.off:b.End]
}

func (b *BufferedStream) ReadByte() (byte, error) {
	if b.empty() {
		_, err := b.Fill()
		if err != nil {
			return 0, err
		}
	}
	r := b.Buf[b.off]
	b.off++
	return r, nil
}

// Fill the buffer by doing one Read() from the underlying reader.
// Calls to Read() will use the buffer.
func (b *BufferedStream) Fill() ([]byte, error) {
	if b.empty() && !b.sniffing {
		b.off = 0
		b.End = 0
	}
	n, err := b.In.Read(b.Buf[b.End:])
	b.End += n
	if err != nil {
		return nil, err
	}
	return b.Buf[b.off:b.End], nil
}

func (b *BufferedStream) Buffer() []byte {
	return b.Buf[b.off:b.End]
}

// Reset will set the bufferRead to off, so Read() will start from there (ignoring
// bytes from header).
// Sniffing mode is disabled.
func (b *BufferedStream) Reset(off int) {
	b.sniffing = false
	b.off = off
}

func (b *BufferedStream) Clean() {
	b.sniffing = false
	b.off = 0
	b.End = 0
}

func (b *BufferedStream) Sniff() {
	b.sniffing = true
	b.off = 0
	b.End = 0
}

func (b *BufferedStream) resetStats() {
	b.Stream = NewStream()
	//b.Meta.Reset()
}

func (b *BufferedStream) Read(p []byte) (int, error) {
	if b.End > b.off {
		// If we have already read something from the buffer before, we return the
		// same data and the last error if any. We need to immediately return,
		// otherwise we may block for ever, if we try to be smart and call
		// source.Read() seeking a little bit of more data.
		bn := copy(p, b.Buf[b.off:b.End])
		b.off += bn
		if !b.sniffing && b.End <= b.off {
			// buffer has been consummed, not in sniff mode
			b.off = 0
			b.End = 0
		}
		return bn, b.lastErr
	}
	// AcceptedCon is reused, keep the buffer around

	// If there is nothing more to return in the sniffed buffer, read from the
	// source.
	sn, sErr := b.In.Read(p)
	if sn > 0 && b.sniffing {
		b.lastErr = sErr
		if len(b.Buf) < b.End+sn {
			return sn, errors.New("short buffer")
		}
		copy(b.Buf[b.End:], p[:sn])
		b.End += sn
	}
	b.Stream.RcvdPackets++
	b.Stream.RcvdBytes += sn
	if sErr != nil {
		b.Stream.ReadErr = sErr
	}
	b.Stream.LastRead = time.Now()
	return sn, sErr
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
func (b *BufferedStream) WriteTo(w io.Writer) (n int64, err error) {
	// Finish up the buffer first
	if !b.empty() {
		bn, err := w.Write(b.Buf[b.off:b.End])
		if err != nil {
			//"Write must return non-nil if it doesn't write the full buffer"
			b.Stream.ProxyWriteErr = err
			return int64(bn), err
		}
		b.off += bn
		n += int64(bn)
	}

	// but the dialed connection might, so we can splice
	if CanSplice(b.In, w) {
		if wt, ok := w.(io.ReaderFrom); ok {
			VarzReadFrom.Add(1)
			n, err = wt.ReadFrom(b.In)
			b.Stream.RcvdPackets++
			b.Stream.RcvdBytes += int(n)
			b.Stream.LastRead = time.Now()
			return
		}
	}

	for {
		sn, sErr := b.In.Read(b.Buf)
		b.Stream.RcvdPackets++
		b.Stream.RcvdBytes += sn

		if sn > int(VarzMaxRead.Value()) {
			VarzMaxRead.Set(int64(sn))
		}

		if sn > 0 {
			wn, wErr := w.Write(b.Buf[0:sn])
			n += int64(wn)
			if wErr != nil {
				b.Stream.ProxyWriteErr = wErr
				return n, wErr
			}
		}
		// May return err but still have few bytes
		if sErr != nil {
			b.Stream.ReadErr = sErr
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
