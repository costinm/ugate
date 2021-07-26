package ugate

import (
	"bytes"
	"encoding/binary"
)

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

// StreamBuffer is an optimized implementation of io.Reader that behaves like
// ```
// io.MultiReader(bytes.NewReader(buffer.Bytes()), io.TeeReader(source, buffer))
// ```
// without allocating.
//
// Also similar with bufio.Reader, but with recycling and access to buffer,
// metadata, stats and for net.Conn.
type StreamBuffer struct {

	// b has end and capacity, set at creation to the size of the buffer
	// using end and off as pointers to data
	// Deprecated - use the methods
	buf []byte

	// read so far from buffer. Unread data in off:End
	off int

	// last bytes with data in the buffer.
	// Deprecated, use the methods
	end int

	// WIP: avoid copy, linked list of buffers.
	//next *StreamBuffer
	// WIP: ownership
	owner interface{}
}

// Reader returns a bytes.Reader for v.
func (v *StreamBuffer) Reader() bytes.Reader {
	var r bytes.Reader
	r.Reset(v.buf[v.off:v.end])
	return r
}

func (b *StreamBuffer) TrimFront(count int)  {
	b.off += count
	if b.off >= b.end {
		b.off = 0
		b.end = 0
	}
}

func (b *StreamBuffer) Recycle() {
	b.owner = bufferedConPool
	bufferedConPool.Put(b)
}

func (b *StreamBuffer) IsEmpty() bool {
	if b== nil {
		return true
	}
	return b.off >= b.end
}

func (b *StreamBuffer) Size() int {
	if b== nil {
		return 0
	}
	return b.end - b.off
}

func (b *StreamBuffer) WriteByte(d byte) {
	b.grow(1)
	b.buf[b.end] = d
	b.end++
}

func (b *StreamBuffer) WriteUnint32(i uint32) {
	b.grow(4)
	binary.LittleEndian.PutUint32(b.buf[b.end:], i)
	b.end += 4
}

func (b *StreamBuffer) WriteVarint(i int64) {
	b.grow(8)
	c := binary.PutVarint(b.buf[b.end:], i)
	b.end += c
}

func (b *StreamBuffer) grow(n int) {
	c := cap(b.buf)
	if c - b.end > n {
		return
	}
	buf := make([]byte, c * 2)
	copy(buf, b.buf[b.off:b.end])
	b.buf = buf
	b.end = b.end - b.off
	b.off = 0
}

func (b *StreamBuffer) Write(p []byte) (n int, err error) {
	n = len(p)
	b.grow(n)
	copy(b.buf[b.end:], p)
	b.end += n
	return
}

// Return the unread portion of the buffer
func (b *StreamBuffer) Bytes() []byte {
	return b.buf[b.off:b.end]
}
