package ugate

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
	"strconv"
)

// Egress capture. See also iptables, TUN, CONNECT proxy
// Should be on a dedicated port.

// curl --socks5 127.0.0.1:15004 ....
// export HTTP_PROXY=socks5://127.0.0.1:15004

const (
	ConnectCommand   = uint8(1)
	BindCommand      = uint8(2)
	AssociateCommand = uint8(3)
	ipv4Address      = uint8(1)
	fqdnAddress      = uint8(3)
	ipv6Address      = uint8(4)
)

const (
	successReply uint8 = iota
	serverFailure
	ruleFailure
	networkUnreachable
	hostUnreachable
	connectionRefused
	ttlExpired
	commandNotSupported
	addrTypeNotSupported
)

const (
	NoAuth          = uint8(0)
	noAcceptable    = uint8(255)
	UserPassAuth    = uint8(2)
	userAuthVersion = uint8(1)
	authSuccess     = uint8(0)
	authFailure     = uint8(1)
)

/*
  RFC1928

  1. Req:
  VER 0x05
  NMETHODS 0x01
  METHOD 0x00 [one byte for each method - NoAuth]
  (other auth not supported - we bind on 127.0.0.1 or use mtls)

  Res:
  VER 0x05
	METHOD 0x00

	2.  VER: X'05'
      CMD
             o  CONNECT X'01'
             o  BIND X'02'
             o  UDP ASSOCIATE X'03'
      RSV    RESERVED 0x00
      ATYP   address type of following address
             o  IP V4 address: X'01'
             o  DOMAINNAME: X'03'
             o  IP V6 address: X'04'
      DST.ADDR       desired destination address
      DST.PORT desired destination port in network octet order
*/

// Extract the target from the SOCKS header, consume it, and register a post-dial
// hook.
func (ug *UGate) sniffSOCKSConn(acceptedCon *RawConn) error {
	head := acceptedCon.buf

	n, err := acceptedCon.Read(head)
	if err != nil {
		acceptedCon.Close()
		log.Println("Failed to read head")
		return err
	}

	// Client: 0x05 0x01 0x00
	//         0x05 0x02  0x00 0x01
	// Server: 0x05 0x00

	if head[0] != 5 {
		log.Print("Unexpected version ", head[0:n])
		acceptedCon.Close()
		return errors.New("Invalid head")
	}

	// 1 method, no auth
	//if head[1] != 1 || head[2] != 0 {
	//	log.Print("Unexpected auth ", head[1], head[2])
	//	return errors.New("Invalid auth")
	//}

	acceptedCon.Write([]byte{5, 0})

	off := 0

	for {
		n, err := acceptedCon.Read(head[off:])
		if err != nil {
			acceptedCon.Close()
			return err
		}
		off += n
		if off < 5 {
			continue
		}

		atyp := head[3]
		switch atyp {
		case 1:
			if off > 10 {
				log.Println("SOCKS: Unexpected extra bytes", off)
			}
			if off < 10 {
				continue
			}
		case 4:
			if off > 22 {
				log.Println("SOCKS: Unexpected extra bytes", off)
			}
			if off < 22 {
				continue
			}
		case 3:
			len := int(head[4])
			if off > len+7 {
				log.Println("SOCKS: Unexpected extra bytes", off)
			}
			if off < len+7 {
				continue
			}
		}
		break
	}
	// TODO: make sure the ip and string are read, read more

	// Client: 0x05 0x01 (connect) 0x00 (RSV) ATYP DADDR DPORT
	cmd := head[1]
	if cmd != 1 {
		log.Println("Only connect is supported")
		acceptedCon.Close()
		return nil
	}

	atyp := head[3]
	var dest net.IP
	var destAddr string
	isString := false
	var port uint16
	// TODO: copy the ip (head will be reused)
	switch atyp {
	case 1:
		dest = net.IP(head[4:8])
		port = binary.BigEndian.Uint16(head[8:])
	case 4:
		dest = net.IP(head[4:20])
		port = binary.BigEndian.Uint16(head[20:])
	case 3:
		isString = true
		len := int(head[4])
		if len == 0 {
			return errors.New("String address too short")
		}
		destAddr = string(head[5 : 5+len])
		port = binary.BigEndian.Uint16(head[5+len:])
	default:
		acceptedCon.Close()
		return errors.New("Unknown address")
	}

	localAddr := acceptedCon.LocalAddr()
	tcpAddr := localAddr.(*net.TCPAddr)

	addr := ""
	ctype := "socks5"
	if isString {
		addr = net.JoinHostPort(destAddr, strconv.Itoa(int(port)))
	} else {
		ta := &net.TCPAddr{IP: dest, Port: int(port)}
		addr = ta.String()
		ctype = "socks5IP"
	}

	acceptedCon.Meta().Request.Host = addr
	acceptedCon.Stream.Type = ctype

	acceptedCon.postDial = func(conn net.Conn, err error) {
		if err != nil {
			// TODO: write error code
			head[1] = 1
			acceptedCon.Write(head[0:2])
			acceptedCon.Close()
		}
		// Not accurate for tcp-over-http.
		// TODO: pass a 'on connect' callback
		r := head[off:]
		r[0] = 5
		r[1] = 0 // success
		r[2] = 0 // rsv
		r[3] = 1 // ip4
		// 4-bytes IP4 local
		copy(r[4:8], []byte(tcpAddr.IP))
		// 2 bytes local port
		binary.BigEndian.PutUint16(r[8:], uint16(tcpAddr.Port))
		acceptedCon.Write(r[0:10])
	}

	acceptedCon.Clean()

	return nil
}
