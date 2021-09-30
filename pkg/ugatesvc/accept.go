package ugatesvc

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/costinm/ugate"
)

// Accepting connections on ports and extracting metadata, including sniffing.

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
func (ug *UGate) HandleTUN(conn net.Conn, ra *net.TCPAddr, la *net.TCPAddr) error {
	bconn := ugate.GetStream(conn, conn)
	bconn.Direction = ugate.StreamTypeOut
	ug.OnStream(bconn)
	defer ug.OnStreamDone(bconn)

	bconn.Session = &ugate.Session{RemoteAddr: ra, LocalAddr: la}

	// Testing/debugging - localhost is captured by table local, rule 0.
	if bconn.Dest == "" {
			bconn.Dest = ra.String()
			log.Println("LTUN ", ra, la, bconn.Dest)
	}

	log.Println("TUN TCP ", bconn)
	// TODO: config ? Could be shared with iptables port
	return ug.HandleStream(bconn)
}

// Handle a virtual (multiplexed) stream, received over
// another connection, for example H2 POST/CONNECT, etc
// The connection will have metadata, may include identify of the caller.
func (ug *UGate) HandleVirtualIN(bconn *ugate.Conn) error {
	ug.OnStream(bconn)
	defer ug.OnStreamDone(bconn)

	return ug.HandleStream(bconn)
}

// handleSNI is intended for a dedicated SNI port.
// Will use the Config.Routes to map the SNI host to a ForwardTo address. If not found,
// will use a callback for a dynamic route.
func (ug *UGate) handleSNI(str *ugate.Conn) error {
	// Used to present the right cert
	_, str.ReadErr = ParseTLS(str)
	if str.ReadErr != nil {
		return str.ReadErr
	}


	route := ug.Config.Routes[str.Dest]
	if route != nil {
		if route.ForwardTo != "" {
			str.Dest = route.ForwardTo
		}
		if route.Handler != nil {
			// SOCKS and others need to send something back - we don't
			// have a real connection, faking it.
			str.PostDial(str, nil)
			str.Dest = fmt.Sprintf("%v", route.Handler)
			err:= route.Handler.Handle(str)
			str.Close()
			return err
		}
	}

	// Default stream handling is proxy to the SNI dest.
	// Note that SNI does not include port number.
	str.ReadErr = ug.DialAndProxy(str)
	return nil
}

// handles a directly accepted TCP connection for a TLS port.
// May SNI-forward or terminate, based on listener config.
//
// If terminating, based on ALPN and domain will route the stream.
// For SNI - will use the SNI name to route the stream.
//
// TODO: depreacte, too complex. Dedicated port for SNI is better and cleaner.
func (ug *UGate) handleTLSorSNI(rawStream *ugate.Conn) error {

	// Used to present the right cert
	_, rawStream.ReadErr = ParseTLS(rawStream)
	if rawStream.ReadErr != nil {
		return rawStream.ReadErr
	}

	sni := rawStream.Dest

	tlsTerm := false
	cert := ""

	if rawStream.Dest == "" {
		// No explicit destination - terminate here
		tlsTerm = true
	} else if rawStream.Dest == ug.Auth.ID {
		tlsTerm = true
	} else {
		// Not sure if this is worth it, may be too complex
		// TODO: try to find certificate for domain or parent.
		//if ug.Auth.Config.Get("key/" + dest) {
		//
		//}
		_, ok := ug.Auth.CertMap[sni]
		if ok {
			tlsTerm = true
			cert = sni
		} else {
			// Check certs defined for the listener
			wild := ""
			for cn, k := range rawStream.Listener.Certs {
				if cn == "*" {
					wild = k
				} else {
					if cn[0] == '*' && len(cn) > 2 {
						if strings.HasSuffix(sni, cn[2:]) {
							tlsTerm = true
							cert = k
						}
					} else if sni == cn {
						tlsTerm = true
						cert = k
					}
				}
			}
			if !tlsTerm && wild != "" {
				tlsTerm = true
				cert = wild
			}
		}
	}

	// At this point, if tlsTerm is true it means we should terminate
	// and handle the connection.
	// Else - SNI forward

	// TODO: local routes if tlsTerm
	sniCfg := ug.Config.Routes[rawStream.Dest]
	if sniCfg != nil {
		if sniCfg.ForwardTo != "" {
			rawStream.Dest = sniCfg.ForwardTo
		}
	}

	// Terminate TLS if the stream is detected as TLS and the matched config
	// is configured for termination.
	// Else it's just a proxied SNI connection.
	if tlsTerm {
		tlsCfg := ug.TLSConfig
		if cert != "" {
			// explicit cert based on listener config
			// TODO: tlsCfg = ug.Auth.GetServerConfig(cert)
		}
		// TODO: present the right ALPN for the port ( if not set, use default)
		tc, err := ug.NewTLSConnIn(rawStream.Context(), rawStream.Listener, rawStream, tlsCfg)
		if err != nil {
			rawStream.ReadErr = err
			log.Println("TLS: ", rawStream.RemoteAddr(), rawStream.Dest, rawStream.Route, err)
			return err
		}

		// Handshake done. Now we have access to the ALPN.
		tc.PostDial(tc, nil)
		rawStream.ReadErr = ug.H2Handler.HandleHTTPS(tc)
	} else {
		// Default stream handling is proxy to the SNI dest.
		// Note that SNI does not include port number.


		rawStream.ReadErr = ug.HandleStream(rawStream)
	}
	return nil
}


// Hamdle implements the common interface for handling accepted streams.
// Will init and log the stream, then handle.
//
func (ug *UGate) Handle(s *ugate.Conn) {
	ug.OnStream(s)
	defer ug.OnStreamDone(s)

	ug.HandleStream(s)
}

// A real accepted connection on a 'legacy' port. Will be forwarded to
// the mesh.
func (ug *UGate) handleTCPForward(bconn *ugate.Conn) error {
	bconn.Direction = ugate.StreamTypeForward

	if bconn.Listener.ForwardTo != "" {
		bconn.Dest = bconn.Listener.ForwardTo
	}

	bconn.ReadErr = ug.HandleStream(bconn)
	return bconn.ReadErr
}

func (ug *UGate) handleTCPEgress(bconn *ugate.Conn) error {
	bconn.Direction = ugate.StreamTypeOut

	bconn.Dest = bconn.Listener.ForwardTo

	bconn.ReadErr = ug.HandleStream(bconn)
	return bconn.ReadErr
}

// Auto-detect protocol on the wire, so routing info can be
// extracted:
// - TLS ( 22, 3)
// - HTTP
// - WS ( CONNECT )
// - SOCKS (5)
// - H2
// - TODO: HAproxy
//func sniff(pl *ugate.Listener, br *stream.StreamBuffer) error {
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
//	br.Conn.Type = proto
//
//	return nil
//}

