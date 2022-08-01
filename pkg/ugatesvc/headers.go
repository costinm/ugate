package ugatesvc

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"log"
	"net/http"
	"net/textproto"

	"github.com/costinm/ugate"
)

// There are few options on how to pass stream metadata around:
//
// Mux proto:
// - H2 - clear protocol, but has overhead and complexity and because
// of muxing we can't splice
//
// Splice-able proto:
// - use HTTP/1.1 CONNECT - and mime headers
// - use HA Proxy - custom binary
// - use a proto (possibly with simplified proto parsing), like Istio
// - ???
//
// A mixed mode is also possible - proto in a h2 or http header.
// Encoding/decoding speed and memory use are the key - technically
// all options can be supported.

type MimeEncoder struct {
}

func (*MimeEncoder) Unmarshal(s *ugate.Stream) (done bool, err error) {
	buf, err := s.Fill(5)
	if err != nil {
		return false, err
	}

	len := binary.LittleEndian.Uint32(buf[1:])
	if len > 32*1024 {
		return false, errors.New("header size")
	}

	buf, err = s.Fill(5 + int(len))
	if err != nil {
		return false, err
	}

	hr := textproto.NewReader(bufio.NewReader(bytes.NewBuffer(buf[5:])))
	mh, err := hr.ReadMIMEHeader()
	s.InHeader = http.Header(mh)

	// Skip the headers -
	s.Skip(5 + int(len))

	if ugate.DebugClose {
		log.Println("Stream.receiveHeaders ", s.StreamId, s.InHeader, s.RBuffer().Size())
	}

	return true, nil
}

func (*MimeEncoder) Marshal(s *ugate.Stream) error {
	bb := s.WBuffer()
	h := s.InHeader
	if s.Direction == ugate.StreamTypeOut ||
		s.Direction == ugate.StreamTypeUnknown {
		h = s.OutHeader
	}

	bb.WriteByte(2) // To differentiate from regular H3, using 0
	bb.Write([]byte{0, 0, 0, 0})
	err := s.OutHeader.Write(bb)
	data := bb.Bytes()

	binary.LittleEndian.PutUint32(data[1:], uint32(bb.Size()-5))
	if err != nil {
		return err
	}

	_, err = s.Write(data)

	if err != nil {
		return err
	}

	if ugate.DebugClose {
		log.Println("Stream.sendHeaders ", s.StreamId, h)
	}

	return nil
}

// BEncoder is a header encoder using a compact encoding, inspired
// from CBOR, CoAP, FlatBuffers.
//
// It is an experiment on evaluating impact of header format on
// latency for small requests (streams).
//
// I am trying to keep the actual encoding wire-compatible with protobuf,
// but with a careful proto:
//  - flat - to avoid the length-prefix
//  - short tags - look like 1 byte tag in other encodings
//  - 'Start' and 'end' fields - regular fields used as delimiters.
//    Could also use (deprecated) Start(3)/stop(4) group.
//   See https://groups.google.com/g/protobuf/c/UKpsthqAmjw for benefits
//   of the deprecated group.
//
// Tags use last 3 bits as:
// 0=varint, 1-64fixed, 2=len-delim, 5=32fixed.
//
type BEncoder struct {
}

func (*BEncoder) AddHeader(s *ugate.Stream, k, v []byte) {
	bb := s.WBuffer()
	if bb.Size() == 0 {
		bb.WriteByte(0)
		//bb.WriteByte(0x08) // tag=1, varint
		bb.WriteUnint32(0) // header-Start
	}
	bb.WriteByte(0x12) // tag=2, len-delim
	bb.WriteVarint(int64(len(k)))
	bb.Write(k)
	bb.WriteByte(0x12) // tag=2, len-delim
	bb.WriteVarint(int64(len(v)))
	bb.Write(v)
}

func (*BEncoder) Unmarshal(s *ugate.Stream) (done bool, err error) {
	h, err := s.Fill(5)
	if err != nil {
		return false, err
	}
	if h[0] != 0 {
		return false, errors.New("unexpected delim")
	}

	// WIP

	return true, nil
}

func (*BEncoder) Marshal(s *ugate.Stream) error {
	bb := s.WBuffer()
	h := s.InHeader
	if s.Direction == ugate.StreamTypeOut ||
		s.Direction == ugate.StreamTypeUnknown {
		h = s.OutHeader
	}
	// TODO: leave 5 bytes at Start to reproduce streaming gRPC format
	bb.WriteByte(0x08) // tag=1, varint
	bb.WriteByte(1)    // header-Start

	for k, vv := range h {
		for _, v := range vv {
			bb.WriteByte(0x12) // tag=2, len-delim
			bb.WriteVarint(int64(len(k)))
			bb.Write([]byte(k))
			bb.WriteByte(0x12) // tag=2, len-delim
			bb.WriteVarint(int64(len(v)))
			bb.Write([]byte(v))
		}
	}
	bb.WriteByte(0x08)
	bb.WriteByte(2)

	if ugate.DebugClose {
		log.Println("Stream.sendHeaders ", s.StreamId, h)
	}

	return nil
}
