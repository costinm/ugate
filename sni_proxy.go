package ugate

import (
	"errors"
	"log"
	"strings"
)

// Istio-style SNI proxy.
//
// Used for accepting ingress stream on a public IP and routing them to a mesh node.
//
// Without DNS:
//
// curl https://foo.com/status -k --resolve *:443:1.2.3.4.
//
// With DNS interception - return the address of the SNI host.
//
// For non-mesh names - explicit config must be used, to associate the domain name
// with an identity. This is similar with Istio 'secure naming': each node has a self-generated
// service account, and the Gateway delegates to it (actually: namespace, but it's equivalent)

type clientHelloMsg struct { // 22
	vers                uint16
	random              []byte
	sessionId           []byte
	cipherSuites        []uint16
	compressionMethods  []uint8
	nextProtoNeg        bool
	serverName          string
	ocspStapling        bool
	scts                bool
	supportedPoints     []uint8
	ticketSupported     bool
	sessionTicket       []uint8
	secureRenegotiation []byte
	alpnProtocols       []string
}

var sniErr = errors.New("Invalid TLS")

// TLS extension numbers
const (
	extensionServerName uint16 = 0
)

func (gw *UGate) sniffSNI(acc *RawConn) error {
	buf := acc.buf
	n, err := acc.Read(buf[0:5])
	if err != nil {
		acc.Close()
		return err
	}

	if n < 5 {
		return sniErr
	}

	typ := buf[0] // 22 3 1 2 0
	if typ != 22 {
		return sniErr
	}

	vers := uint16(buf[1])<<8 | uint16(buf[2])
	if vers != 0x301 {
		log.Println("Version ", vers)
	}

	rlen := int(buf[3])<<8 | int(buf[4])
	if rlen > 4096 {
		return sniErr
	}

	off := 5
	m := clientHelloMsg{}

	end := rlen + 5
	for {
		n, err := acc.Read(buf[off:end])
		if err != nil {
			acc.Close()
			return err
		}
		off += n
		if off >= end {
			break
		}
	}
	clientHello := buf[5:end]
	chLen := end - 5

	if chLen < 38 {
		return sniErr
	}

	// off is the last byte in the buffer - will be forwarded

	m.vers = uint16(clientHello[4])<<8 | uint16(clientHello[5])
	// random: data[6:38]

	sessionIdLen := int(clientHello[38])
	if sessionIdLen > 32 || chLen < 39+sessionIdLen {
		return sniErr
	}
	m.sessionId = clientHello[39 : 39+sessionIdLen]
	off = 39 + sessionIdLen

	// cipherSuiteLen is the number of bytes of cipher suite numbers. Since
	// they are uint16s, the number must be even.
	cipherSuiteLen := int(clientHello[off])<<8 | int(clientHello[off+1])
	off += 2
	if cipherSuiteLen%2 == 1 || chLen-off < 2+cipherSuiteLen {
		return sniErr
	}

	//numCipherSuites := cipherSuiteLen / 2
	//m.cipherSuites = make([]uint16, numCipherSuites)
	//for i := 0; i < numCipherSuites; i++ {
	//	m.cipherSuites[i] = uint16(data[2+2*i])<<8 | uint16(data[3+2*i])
	//}
	off += cipherSuiteLen

	compressionMethodsLen := int(clientHello[off])
	off++
	if chLen-off < 1+compressionMethodsLen {
		return sniErr
	}
	//m.compressionMethods = data[1 : 1+compressionMethodsLen]
	off += compressionMethodsLen

	if off+2 > chLen {
		// ClientHello is optionally followed by extension data
		return sniErr
	}

	extensionsLength := int(clientHello[off])<<8 | int(clientHello[off+1])
	off = off + 2
	if extensionsLength != chLen-off {
		return sniErr
	}

	for off < chLen {
		extension := uint16(clientHello[off])<<8 | uint16(clientHello[off+1])
		off += 2
		length := int(clientHello[off])<<8 | int(clientHello[off+1])
		off += 2
		if off >= end {
			return sniErr
		}

		switch extension {
		case extensionServerName:
			d := clientHello[off : off+length]
			if len(d) < 2 {
				return sniErr
			}
			namesLen := int(d[0])<<8 | int(d[1])
			d = d[2:]
			if len(d) != namesLen {
				return sniErr
			}
			for len(d) > 0 {
				if len(d) < 3 {
					return sniErr
				}
				nameType := d[0]
				nameLen := int(d[1])<<8 | int(d[2])
				d = d[3:]
				if len(d) < nameLen {
					return sniErr
				}
				if nameType == 0 {
					m.serverName = string(d[:nameLen])
					// An SNI value may not include a
					// trailing dot. See
					// https://tools.ietf.org/html/rfc6066#section-3.
					if strings.HasSuffix(m.serverName, ".") {
						return sniErr
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

	// TODO: unmangle server name - port, mesh node

	destAddr := m.serverName + ":443"
	acc.Meta().Target = destAddr
	acc.Stats.Type = "sni"

	// Leave all bytes in the buffer, will be sent
	acc.Reset(0)

	return nil
}
