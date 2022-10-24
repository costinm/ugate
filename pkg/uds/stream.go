package uds

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"strings"
)

// Process a stream of messages - framing, parsing.
// Current implementation: 2-byte prefix,
//
// WIP: other formats
// The format is TLV or delimited, based on the first byte.
//
// 0 NNNN - TLV, payload expected to be a proto ( this is the format of streaming gRPC, so it is possible
//   to use it directly in a handler for efficient, complexity-free gRPC)
//
// 1 NNNN - not used, it's gRPC compressed message.
//
// '{' - delimited JSON, \0
//
// \n - delimited JSON, \n
//
// 2 NNNNN - TLV, payload is JSON.
//
// 'event:' - SSE, frame delim: \n\n or \r\n\r\n

// NATS: text based
// PUB and MSG have payload,
// PUB subject reply-to bytecount\r\nCOUNTBYTES\r\n
// subscription id - associate message with SUB subscription
//
// first line: METHOD param:val ...
// 'subject', '

// Send a binary packet, with len prefix.
// Currently used in the UDS mapping.
func SendFrameLenBinary(con io.Writer, data ...[]byte) (int, error) {
	dlen := 0
	for _, d := range data {
		if d == nil {
			continue
		}
		dlen += len(d)
	}

	msg := make([]byte, dlen+5)

	off := 5
	for _, d := range data {
		if d == nil {
			continue
		}
		copy(msg[off:], d)
		off += len(d)
	}
	msg[0] = 0
	binary.LittleEndian.PutUint32(msg[1:], uint32(dlen))

	if con != nil {
		_, err := con.Write(msg)
		if Debug {
			log.Println("Frame N2A: ", len(data), data[0])
		}
		return len(data), err
	}
	return 0, nil
}

// Parse a message.
// Currently used in the UDS mapping, using a HTTP1-like text format
// Matches code in android java code - may be replaced with gRPC proto framing.
func ParseMessage(data []byte, mtype int) (cmd string, meta map[string]string, outd []byte, end int) {
	start := 0
	n := len(data)
	meta = map[string]string{}

	endLine := bytes.IndexByte(data[start:n], '\n')

	if endLine < 0 { // short message, old style
		endLine = n
		cmd = string(data[0:n])
		log.Println("UDS: short", cmd)
		return
	}
	cmd = string(data[0:endLine])
	if Debug {
		log.Println("UDS: cmd", n, endLine, cmd)
	}

	endLine++
	for {
		nextLine := bytes.IndexByte(data[endLine:n], '\n')
		if nextLine == -1 {
			break // shouldn't happen - \n\n expected
		}
		if nextLine == 0 {
			endLine++ // end of headers
			break
		}
		kv := string(data[endLine : endLine+nextLine])
		kvp := strings.SplitN(kv, ":", 2)
		if len(kvp) != 2 {
			continue
		}
		meta[kvp[0]] = kvp[1]
		if Debug {
			log.Println("UDS: key", kvp)
		}
		endLine += nextLine
		endLine++
	}

	if endLine < n {
		outd = data[endLine:n]
	}

	return
}
