package nio2

import (
	"errors"
	"log"
	"strings"
)

var sniErr = errors.New("Invalid TLS")

// ClientHelloMsg is a subset of the TLS ClientHello
type ClientHelloMsg struct {
	Raw []byte

	vers uint16
	//random              []byte
	SessionID    []byte
	CipherSuites []uint16
	//compressionMethods  []uint8
	ServerName string
	//ocspStapling        bool
	//scts                bool
	//supportedPoints     []uint8
	//ticketSupported     bool
	//sessionTicket       []uint8
	//secureRenegotiation []byte
	//AlpnProtocols                    []string
}

// TLS extension numbers
const (
	extensionServerName uint16 = 0
)

// SniffClientHello will peek into acc and read enough for parsing a
// TLS ClientHello. All read data will be left in the stream, including bytes
// after ClientHello.
//
// If ClientHello is not detected or is invalid - nil will be returned.
//
// TODO: if a session WorkloadID is provided, use it as a cookie and attempt
// to find the corresponding host.
// On server side generate session IDs !
//
// TODO: in mesh, use one cypher suite (TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256)
// maybe 2 ( since keys are ECDSA )
func SniffClientHello(acc *BufferReader) (*ClientHelloMsg, string, error) {
	// We consume only 1 byte if the protocol is not TLS
	buf, err := acc.Peek(1)
	if err != nil {
		return nil, "", err
	}
	typ := buf[0] // 22 3 1 2 0
	if typ != 0x16 {
		// Not an error - this is not TLS
		return nil, "", nil
	}

	// Read the rest of the packet header.
	buf, err = acc.Peek(5)
	if err != nil {
		return nil, "", err
	}
	vers := uint16(buf[1])<<8 | uint16(buf[2])
	if vers != 0x301 {
		log.Println("Version ", vers)
	}

	// Record length - capped at 16K
	rlen := int(buf[3])<<8 | int(buf[4])
	if rlen > 16*1024 {
		log.Println("too large ClientHello", rlen)
		return nil, "", sniErr
	}
	if rlen < 38 {
		log.Println("too small ClientHello ", rlen)
		return nil, "", sniErr
	}

	// Peek the full record - will do multiple reads, may go beyond the first packet.
	end := rlen + 5
	buf, err = acc.Peek(end)
	if err != nil {
		return nil, "", err
	}
	clientHello := buf[5:end]
	m := ClientHelloMsg{Raw: clientHello}

	m.vers = uint16(clientHello[4])<<8 | uint16(clientHello[5])
	// random: data[6:38]

	sessionIdLen := int(clientHello[38])
	if sessionIdLen > 32 || rlen < 39+sessionIdLen {
		return nil, "", sniErr
	}
	m.SessionID = clientHello[39 : 39+sessionIdLen]
	off := 39 + sessionIdLen

	// cipherSuiteLen is the number of bytes of cipher suite numbers. Since
	// they are uint16s, the number must be even.
	cipherSuiteLen := int(clientHello[off])<<8 | int(clientHello[off+1])
	off += 2
	if cipherSuiteLen%2 == 1 || rlen-off < 2+cipherSuiteLen {
		return nil, "", sniErr
	}

	numCipherSuites := cipherSuiteLen / 2
	m.CipherSuites = make([]uint16, numCipherSuites)
	for i := 0; i < numCipherSuites; i++ {
		m.CipherSuites[i] = uint16(clientHello[off+2*i])<<8 | uint16(clientHello[off+1+2*i])
	}
	off += cipherSuiteLen

	compressionMethodsLen := int(clientHello[off])
	off++
	if rlen-off < 1+compressionMethodsLen {
		return nil, "", sniErr
	}
	//m.compressionMethods = data[1 : 1+compressionMethodsLen]
	off += compressionMethodsLen

	if off+2 > rlen {
		// ClientHello is optionally followed by extension data
		return nil, "", sniErr
	}

	extensionsLength := int(clientHello[off])<<8 | int(clientHello[off+1])
	off = off + 2
	if extensionsLength != rlen-off {
		return nil, "", sniErr
	}

	for off < rlen {
		extension := uint16(clientHello[off])<<8 | uint16(clientHello[off+1])
		off += 2
		length := int(clientHello[off])<<8 | int(clientHello[off+1])
		off += 2
		if off >= end {
			return nil, "", sniErr
		}

		switch extension {
		case extensionServerName:
			d := clientHello[off : off+length]
			if len(d) < 2 {
				return nil, "", sniErr
			}
			namesLen := int(d[0])<<8 | int(d[1])
			d = d[2:]
			if len(d) != namesLen {
				return nil, "", sniErr
			}
			for len(d) > 0 {
				if len(d) < 3 {
					return nil, "", sniErr
				}
				nameType := d[0]
				nameLen := int(d[1])<<8 | int(d[2])
				d = d[3:]
				if len(d) < nameLen {
					return nil, "", sniErr
				}
				if nameType == 0 {
					m.ServerName = string(d[:nameLen])
					// An SNI value may not include a
					// trailing dot. See
					// https://tools.ietf.org/html/rfc6066#section-3.
					if strings.HasSuffix(m.ServerName, ".") {
						return nil, "", sniErr
					}
					break
				}
				d = d[nameLen:]
			}
		default:
			//log.Println("TLS Ext", extension, length)
		}

		off += length
	}

	// Does not contain port !!! Assume the port is 443, or map it.

	return &m, m.ServerName, nil
}
