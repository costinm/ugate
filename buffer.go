package ugate

// From: github.com/soheilhy/cmux

// Copyright 2016 The CMux Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

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
	return &AcceptedConn{buf: make([]byte, bufSize)}
}}

func GetConn(in net.Conn) *AcceptedConn {
	br := bufferedConPool.Get().(*AcceptedConn)
	br.Conn = in
	br.ResetStats()
	return br
}

// Wraps accepted connections, keeps a buffer and detected metadata.
//
// AcceptedConn is an optimized implementation of io.Reader that behaves like
// ```
// io.MultiReader(bytes.NewReader(buffer.Bytes()), io.TeeReader(source, buffer))
// ```
// without allocating.
//
// Also similar with bufio.Reader, but with recycling and access to buffer,
// metadata, stats and for net.Conn.
// TODO: use net.Buffers ? The net connection likely implements it.
type AcceptedConn struct {
	// Typically a *net.TCPConn, implements ReaderFrom
	net.Conn

	// if true, anything read will be added to buffer.
	// if false, Read() will consume the buffer from off to len, then use
	// direct Read.
	sniffing   bool

	// b has len and capacity, set at creation to the size of the buffer,
	// len(buf) == cap(buf) == 8k
	// using len and off as pointers to data
	buf []byte

	// read so far from buffer. Unread data in off:last
	off int

	// number of bytes in buffer.
	len int

	// If an error happened while sniffing
	lastErr    error


	// Target address, from config or protocol (Socks, SNI, etc)
	// host:port or any other form accepted by DialContext
	Target string

	// Optional function to call after dial. Used to send metadata
	// back to the protocol ( for example SOCKS)
	postDial func(net.Conn, error)

	Stats Stats
}

func (b *AcceptedConn) Write(p []byte) (int, error) {
	n, err := b.Conn.Write(p)
	if err != nil {
		b.Stats.WriteErr = err
		return n, err
	}
	b.Stats.WritePackets++
	b.Stats.WriteBytes+= n
	b.Stats.LastWrite = time.Now()
	return n, err
}
func (b *AcceptedConn) empty() bool {
	return b.off >= b.len
}

func (b *AcceptedConn) Len() int {
	//
	return b.len - b.off
}

// Return the unread portion of the buffer
func (b *AcceptedConn) Bytes() []byte {
	return b.buf[b.off:b.len]
}

func (b *AcceptedConn) ReadByte() (byte, error) {
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
func (b *AcceptedConn) Fill() (error) {
	if b.empty() && !b.sniffing {
		b.off = 0
		b.len = 0
	}
	n, err := b.Conn.Read(b.buf[b.len:])
	b.len += n
	if err != nil {
		return err
	}
	return nil
}

func (b *AcceptedConn) Read(p []byte) (int, error) {
	if b.len > b.off {
		// If we have already read something from the buffer before, we return the
		// same data and the last error if any. We need to immediately return,
		// otherwise we may block for ever, if we try to be smart and call
		// source.Read() seeking a little bit of more data.
		bn := copy(p, b.buf[b.off:b.len])
		b.off += bn
		if !b.sniffing && b.len <= b.off {
			// buffer has been consummed, not in sniff mode
			b.off = 0
			b.len = 0
		}
		return bn, b.lastErr
	}
	// AcceptedCon is reused, keep the buffer around

	// If there is nothing more to return in the sniffed buffer, read from the
	// source.
	sn, sErr := b.Conn.Read(p)
	if sn > 0 && b.sniffing {
		b.lastErr = sErr
		if len(b.buf) < b.len+ sn {
			return sn, errors.New("Short buffer")
		}
		copy(b.buf[b.len:], p[:sn])
		b.len += sn
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
func (b *AcceptedConn) Reset(off int) {
	b.sniffing = false
	b.off = off
}

func (b *AcceptedConn) Clean() {
	b.sniffing = false
	b.off = 0
	b.len = 0
}

func (b *AcceptedConn) Sniff() {
	b.sniffing = true
	b.off = 0
	b.len = 0
}

func (b *AcceptedConn) ResetStats() {
	b.Stats.Reset()
}

// Proxy the accepted connection to a dialed connection.
// Blocking, will wait for both sides to FIN or RST.
func (b *AcceptedConn) Proxy(cl net.Conn) error {
	errCh := make(chan error, 2)

	go b.ProxyFromClient(cl, errCh)

	return b.ProxyToClient(cl, errCh)
}

// WriteTo implements the interface, using the read buffer.
func (b *AcceptedConn) WriteTo(w io.Writer) (n int64, err error) {
	// Finish up the buffer first
	if !b.empty() {
		bn, err := w.Write(b.buf[b.off:b.len])
		if err != nil {
			//"Write must return non-nil if it doesn't write the full buffer"
			b.Stats.ProxyWriteErr = err
			return int64(bn), err
		}
		b.off += bn
		n += int64(bn)
	}

	// Tcp connections don't typically implement WriterTo -
	// but the dialed connection might, so we can splice
	if _, ok := w.(*net.TCPConn); ok {
		if _, ok := b.Conn.(*net.TCPConn); ok {
			if wt, ok := w.(io.ReaderFrom); ok {
				VarzReadFrom.Add(1)
				return wt.ReadFrom(b.Conn)
			}
		}
	}

	for {
		sn, sErr := b.Conn.Read(b.buf)
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
func (b *AcceptedConn) ProxyToClient(cin io.Writer, errch chan error) error {
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

// Reads data from cin (the client/dialed con) until EOF or error
// TCP Connections typically implement this, using io.Copy().
func (b *AcceptedConn) ReadFrom(cin io.Reader) (n int64, err error) {
	// Typical case - accepted connections are TCPConn and implement
	// this efficiently
	// However ReadFrom fallbacks to Copy without recycling the buffer
	//
	if _, ok := cin.(*os.File); ok {
		if _, ok := b.Conn.(*net.TCPConn); ok {
			if wt, ok := b.Conn.(io.ReaderFrom); ok {
				VarzReadFromC.Add(1)
				return wt.ReadFrom(cin)
			}
		}
	}

	if _, ok := cin.(*net.TCPConn); ok {
		if _, ok := b.Conn.(*net.TCPConn); ok {
			if wt, ok := b.Conn.(io.ReaderFrom); ok {
				VarzReadFromC.Add(1)
				return wt.ReadFrom(cin)
			}
		}
	}
	if _, ok := cin.(*net.UnixConn); ok {
		if wt, ok := b.Conn.(io.ReaderFrom); ok {
			return wt.ReadFrom(cin)
		}
	}
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
		if nr > int(VarzMaxRead.Value()) {
			VarzMaxRead.Set(int64(nr))
		}

		_, err := b.Conn.Write(buf[0:nr])
		if err != nil {
			return n, err
		}
	}

	return
}

// ProxyFromClient writes to the net.Conn. Should be in a go routine.
func (b *AcceptedConn) ProxyFromClient(cin io.Reader, errch chan error)  {
	_, err := b.ReadFrom(cin)

	// At this point either cin returned FIN or RST

	if cw, ok := b.Conn.(CloseWriter); ok {
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
