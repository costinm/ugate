package ugatesvc

import (
	"log"
	"net"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/sni"
	"github.com/costinm/ugate/pkg/socks"
)

// Accepting connections on ports and extracting metadata, including sniffing.



// Deferred to connection close, only for the raw accepted connection.
func (ug *UGate) onAcceptDoneAndRecycle(rc *ugate.BufferedStream) {
	ug.OnStreamDone(rc)
	ugate.BufferedConPool.Put(rc)
}

// WIP: special handling for egress, i.e. locally originated streams.
// Identification:
// - dedicated listener ports for iptables, socks5 or tun
// - listeners with address 127.0.0.1
// - connections with src/dst address in 127.0.0.0/8
//
// The last 2 are 'whitebox' mode, using the Port and address to select
// the routes.
//
// After determining the target from meta or config the request is proxied.
//func (pl *Listener) handleEgress(acceptedCon net.Conn) error {
//
//	return nil
//}

// WIP: handling for accepted connections for this node.
//func (pl *Listener) handleLocal(acceptedCon net.Conn) error {
//
//	return nil
//}

// For BTS/H2 and iptables-in, the config for the actual listen port is virtual.
// LocalAddr port determines which config to use for routing.
// RemoteAddr is the (authenticated) remote VIP or the real client IP.
//
// TUN and iptables(out) are used for egress, remoteAddr is the destination on a remote
// computer, and localAddr is on same computer and not very useful.

// New style, based on lwip. Blocks until connect, proxy runs in background.
func (ug *UGate) HandleTUN(conn net.Conn, target *net.TCPAddr) error {
	bconn := ugate.GetConn(conn)
	bconn.Meta().Egress = true
	ug.OnStream(bconn.Meta())
	defer ug.OnStreamDone(bconn)

	ra := conn.RemoteAddr()
	//la := conn.LocalAddr()

	// Testing/debugging - localhost is captured by table local, rule 0.
	if bconn.Stream.Dest == "" {
		if ta, ok := ra.(*net.TCPAddr); ok {
			if ta.Port == 5201 {
				ta.IP = []byte{0x7f, 0, 0, 1}
			}
			bconn.Stream.Dest = ta.String()
			log.Println("LTUN ", conn.RemoteAddr(), conn.LocalAddr(), bconn.Stream.Dest)
		}
	}
	bconn.Stream.Egress = true

	log.Println("TUN TCP ", bconn.Meta())
	// TODO: config ? Could be shared with iptables port
	return ug.HandleStream(bconn.Meta())
}

// Handle a virtual (multiplexed) stream, received over
// another connection, for example H2 POST/CONNECT, etc
// The connection will have metadata, may include identify of the caller.
func (ug *UGate) HandleVirtualIN(bconn ugate.MetaConn) error {
	ug.OnStream(bconn.Meta())
	defer ug.OnStreamDone(bconn)

	return ug.HandleStream(bconn.Meta())
}


// A real accepted connection from port_listener.
// Based on config, sniff and possibly dispatch to a different type.
func (ug *UGate) handleAcceptedConn(l *ugate.Listener, acceptedCon net.Conn) {
	// Attempt to determine the Listener and target
	// Ingress mode, forward to an IP
	cfg := l

	tlsTerm := false

	// Get a buffered stream - this is used for sniffing.
	// Most common case is TLS, we want the SNI.
	bconn := ugate.GetConn(acceptedCon)
	ug.OnStream(bconn.Meta())
	defer ug.OnStreamDone(bconn)
	str := bconn.Meta()

	// Special protocols, muxed on a single port - will extract real
	// destination and port.
	//
	// First are specific to egress capture.
	switch cfg.Protocol {
	// TODO: costin: does not compile on android gomobile, missing syscall.
	// remove dep, reverse it.
	case ugate.ProtoSocks:
		str.Egress = true
		str.ReadErr = socks.ReadSocksHeader(bconn)
	case ugate.ProtoHTTP:
		str.ReadErr = ug.H2Handler.handleHTTPListener(l, bconn)
		return
	case ugate.ProtoHTTPS:
		tlsTerm = true
		// Used to present the right cert
		str.ReadErr = sni.SniffSNI(bconn)
		if str.ReadErr != nil {
			log.Println("XXX Failed to snif SNI", str.ReadErr)
		}
	case ugate.ProtoTLS:
		str.ReadErr = sni.SniffSNI(bconn)
		if str.ReadErr != nil {
			log.Println("XXX Failed to snif SNI in TLS", str.ReadErr, l.Address, l.Protocol, bconn.Meta())
		}
		if str.Dest == "" {
			// No destination - terminate here
			// TODO: also if dest hostname == local name or VIP or ID
			tlsTerm = true
			str.Dest = l.ForwardTo
		}
		// TODO: https + TLS - if we have a cert, accept it
		// Else - forward based on forwardTo or Host
		//if ug.Auth.certMap[str.Dest] != nil {
		//	tlsTerm = true
		//}
	}
	str.Listener = cfg
	str.Type = cfg.Protocol

	if str.ReadErr != nil {
		return
	}

	if cfg.ForwardTo != "" {
		// Override - for explicit ports and virtual ports
		// At listener level.
		str.Dest = cfg.ForwardTo
	}

	tlsOrOrigStr := str
	// Terminate TLS if the stream is detected as TLS and the matched config
	// is configured for termination.
	// Else it's just a proxied SNI connection.
	if tlsTerm {
		tc, err := ug.NewTLSConnIn(str.Context(),cfg,  bconn, ug.TLSConfig)
		if err != nil {
			str.ReadErr = err
			log.Println("TLS: ", err)
			return
		}
		tlsOrOrigStr = tc.Meta()
	}

	str.ReadErr = ug.HandleStream(tlsOrOrigStr)
}

// Auto-detect protocol on the wire, so routing info can be
// extracted:
// - TLS ( 22, 3)
// - HTTP
// - WS ( CONNECT )
// - SOCKS (5)
// - H2
// - TODO: HAproxy
//func sniff(pl *ugate.Listener, br *stream.BufferedStream) error {
//	br.Sniff()
//	var proto string
//
//	for {
//		_, err := br.Fill()
//		if err != nil {
//			return err
//		}
//		off := br.end
//		if off >= 2 {
//			b0 := br.buf[0]
//			b1 := br.buf[1]
//			if b0 == 5 {
//				proto = ProtoSocks
//				break
//			}
//			// TLS or SNI - based on the hostname we may terminate locally !
//			if b0 == 22 && b1 == 3 {
//				// 22 03 01..03
//				proto = ProtoTLS
//				break
//			}
//		}
//		if off >= 7 {
//			if bytes.Equal(br.buf[0:7], []byte("CONNECT")) {
//				proto = ProtoConnect
//				break
//			}
//		}
//		if off >= len(h2ClientPreface) {
//			if bytes.Equal(br.buf[0:len(h2ClientPreface)], h2ClientPreface) {
//				proto = ProtoH2
//				break
//			}
//		}
//	}
//
//	// All bytes in the buffer will be Read again
//	br.Reset(0)
//
//	switch proto {
//	case ProtoSocks:
//		pl.gate.readSocksHeader(br)
//	case ProtoTLS:
//		pl.gate.sniffSNI(br)
//	}
//	br.Stream.Type = proto
//
//	return nil
//}

