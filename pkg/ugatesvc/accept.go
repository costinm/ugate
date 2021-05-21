package ugatesvc

import (
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
	bconn.Egress = true
	ug.OnStream(bconn)
	defer ug.OnStreamDone(bconn)

	bconn.Session = &ugate.Session{RemoteAddr: ra, LocalAddr: la}

	// Testing/debugging - localhost is captured by table local, rule 0.
	if bconn.Dest == "" {
			bconn.Dest = ra.String()
			log.Println("LTUN ", ra, la, bconn.Dest)
	}
	bconn.Egress = true

	log.Println("TUN TCP ", bconn)
	// TODO: config ? Could be shared with iptables port
	return ug.HandleStream(bconn)
}

// Handle a virtual (multiplexed) stream, received over
// another connection, for example H2 POST/CONNECT, etc
// The connection will have metadata, may include identify of the caller.
func (ug *UGate) HandleVirtualIN(bconn *ugate.Stream) error {
	ug.OnStream(bconn)
	defer ug.OnStreamDone(bconn)

	return ug.HandleStream(bconn)
}

// handles a directly accepted TCP connection for a TLS port.
// May SNI-forward or terminate, based on listener config.
//
// If terminating, based on ALPN and domain will route the stream.
// For SNI - will use the SNI name to route the stream.
func (ug *UGate) handleTLS(l *ugate.Listener, acceptedCon net.Conn) {
	// Attempt to determine the Listener and target
	// Ingress mode, forward to an IP

	rawStream := ugate.GetStream(acceptedCon, acceptedCon)

	// Track the original stream. Report error and bytes on the original.
	ug.OnStream(rawStream)
	defer ug.OnStreamDone(rawStream)

	rawStream.Listener = l
	rawStream.Type = l.Protocol


	// Used to present the right cert
	_, rawStream.ReadErr = ParseTLS(rawStream)
	if rawStream.ReadErr != nil {
		return
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
			for cn, k := range l.Certs {
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

	sniCfg := ug.Listeners[rawStream.Dest]
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
		tc, err := ug.NewTLSConnIn(rawStream.Context(),l, rawStream, tlsCfg)
		if err != nil {
			rawStream.ReadErr = err
			log.Println("TLS: ", rawStream.RemoteAddr(), rawStream.Dest, rawStream.Listener, err)
			return
		}

		// Handshake done. Now we have access to the ALPN.
		rawStream.ReadErr = ug.HandleBTSStream(tc)
	} else {
		// Default stream handling is proxy.
		rawStream.ReadErr = ug.HandleStream(rawStream)
	}
}

// Dedicated BTS handler, for accepted connections with TLS.
// Port 443 (if root or redirected), or BASE + 7
//
// curl https://$NAME/ --connect-to $NAME:443:127.0.0.1:15007
func (ug *UGate) handleBTS(l *ugate.Listener, acceptedCon net.Conn) {
	// Attempt to determine the Listener and target
	// Ingress mode, forward to an IP

	rawStream := ugate.GetStream(acceptedCon, acceptedCon)

	// Track the original stream. Report error and bytes on the original.
	ug.OnStream(rawStream)
	defer ug.OnStreamDone(rawStream)

	rawStream.Listener = l
	rawStream.Type = l.Protocol


	// Used to present the right cert
	_, rawStream.ReadErr = ParseTLS(rawStream)
	if rawStream.ReadErr != nil {
		return
	}

	sni := rawStream.Dest

	// 2 main cases:
	// - terminated here - if we have a cert
	// - SNI routed - no termination.
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
		}
	}

	sniCfg := ug.Listeners[rawStream.Dest]
	if sniCfg == nil {
		idx :=  strings.Index(rawStream.Dest, ".")
		if idx > 0 {
			sniCfg = ug.Listeners[rawStream.Dest[0:idx]]
		}
	}
	if sniCfg != nil {
		if sniCfg.ForwardTo != "" {
			rawStream.Dest = sniCfg.ForwardTo
		}
	}

	if l.ForwardTo != "" {
		// Explicit override - this is for listeners with type TLS and explicit
		// forward, for example terminating MySQL.
		// We still terminate TLS if we have a cert.
		rawStream.Dest = l.ForwardTo
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
		tc, err := ug.NewTLSConnIn(rawStream.Context(),l, rawStream, tlsCfg)
		if err != nil {
			rawStream.ReadErr = err
			log.Println("TLS: ", rawStream.RemoteAddr(), rawStream.Dest, rawStream.Listener, err)
			return
		}

		// Handshake done. Now we have access to the ALPN.

		rawStream.ReadErr = ug.HandleBTSStream(tc)
	} else {
		// Default stream handling is proxy.
		rawStream.ReadErr = ug.HandleSNIStream(rawStream)
	}
}

// Hamdle implements the common interface for handling accepted streams.
// Will init and log the stream, then handle.
//
func (ug *UGate) Handle(s *ugate.Stream) {
	ug.OnStream(s)
	defer ug.OnStreamDone(s)

	ug.HandleStream(s)
}

// A real accepted connection from port_listener - a real port, typically for
// legacy protocols and 'whitebox'.
func (ug *UGate) handleAcceptedConn(cfg *ugate.Listener, acceptedCon net.Conn) {

	bconn := ugate.GetStream(acceptedCon, acceptedCon)
	bconn.Listener = cfg
	bconn.Type = cfg.Protocol

	ug.OnStream(bconn)
	defer ug.OnStreamDone(bconn)

	if cfg.ForwardTo != "" {
		bconn.Dest = cfg.ForwardTo
	}

	bconn.ReadErr = ug.HandleStream(bconn)
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
//	br.Stream.Type = proto
//
//	return nil
//}

