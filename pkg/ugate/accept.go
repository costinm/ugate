package ugate

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"runtime/debug"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/iptables"
	"github.com/costinm/ugate/pkg/sni"
	"github.com/costinm/ugate/pkg/socks"
)

// Accepting connections on ports and extracting metadata, including sniffing.
//

// Called at the end of the connection handling. After this point
// nothing should use or refer to the connection, both proxy directions
// should already be closed for write or fully closed.
func (ug *UGate) onAcceptDone(rc ugate.MetaConn) {
	str := rc.Meta()
	ug.m.Lock()
	delete(ug.ActiveTcp, str.StreamId)
	ug.m.Unlock()
	ugate.TcpConActive.Add(-1)
	// TODO: track multiplexed streams separately.
	if str.ReadErr != nil {
		ugate.VarzSErrRead.Add(1)
	}
	if str.WriteErr != nil {
		ugate.VarzSErrWrite.Add(1)
	}
	if str.ProxyReadErr != nil {
		ugate.VarzCErrRead.Add(1)
	}
	if str.ProxyWriteErr != nil {
		ugate.VarzCErrWrite.Add(1)
	}

	if r := recover(); r != nil {

		debug.PrintStack()

		// find out exactly what the error was and set err
		var err error

		switch x := r.(type) {
		case string:
			err = errors.New(x)
		case error:
			err = x
		default:
			err = errors.New("Unknown panic")
		}
		log.Println("AC: Recovered in f", r, err)
	}

	if str.ReadErr != io.EOF && str.ReadErr != nil ||
		str.WriteErr != nil {
		log.Println("Err in:", str.ReadErr, str.WriteErr)
	}
	if str.ProxyReadErr != nil || str.ProxyWriteErr != nil {
		log.Println("Err out:", str.ProxyReadErr, str.ProxyWriteErr)
	}
	if !str.Closed {
		str.Close()
	}

	log.Printf("AC: %d src=%s://%v dst=%s rcv=%d/%d snd=%d/%d la=%v ra=%v op=%v",
		str.StreamId,
		str.Type, rc.RemoteAddr(),
		str.Dest,
		str.RcvdPackets, str.RcvdBytes,
		str.SentPackets, str.SentBytes,
		time.Since(str.LastWrite),
		time.Since(str.LastRead),
		int64(time.Since(str.Open).Seconds()))
	if bc, ok := rc.(*ugate.RawConn); ok {
		ugate.BufferedConPool.Put(bc)
	}
}

// Deferred to connection close, only for the raw accepted connection.
func (ug *UGate) onAcceptDoneAndRecycle(rc *ugate.RawConn) {
	ug.onAcceptDone(rc)
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

func (ug *UGate) HandleUdp(dstAddr net.IP, dstPort uint16, localAddr net.IP, localPort uint16, data []byte) {

}

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
	ug.trackStreamIN(bconn.Meta())
	defer ug.onAcceptDone(bconn)

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

	// TODO: config ? Could be shared with iptables port
	return ug.handleStream(bconn.Meta())
}

// Handle a virtual (multiplexed) stream, received over
// another connection.
func (ug *UGate) HandleVirtualIN(bconn ugate.MetaConn) error {
	ug.trackStreamIN(bconn.Meta())
	defer ug.onAcceptDone(bconn)

	return ug.handleStream(bconn.Meta())
}

// At this point the stream has the metadata:
// - RequestURI
// - Host
// - Headers
// - TLS context
// - Dest and Listener are set.
func (ug *UGate) handleStream(str *ugate.Stream) error {
	if str.Listener == nil {
		str.Listener = ug.DefaultListener
	}
	cfg := str.Listener

	if cfg.Protocol == ugate.ProtoHTTPS {
		str.PostDial(str, nil)
		return ug.h2Handler.Handle(str)
	}

	// Config has an in-process handler - not forwarding (or the handler may
	// forward).
	if cfg.Handler != nil {
		// SOCKS and others need to send something back - we don't
		// have a real connection, faking it.
		str.PostDial(str, nil)
		return cfg.Handler.Handle(str)
	}

	// By default, dial out
	return ug.dialOut(str)
}

func (ug *UGate) trackStreamIN(s *ugate.Stream) {
	ug.m.Lock()
	ug.ActiveTcp[s.StreamId] = s
	ug.m.Unlock()

	ugate.TcpConActive.Add(1)
	ugate.TcpConTotal.Add(1)
}

// A real accepted connection. Based on config, sniff and possibly
// dispatch to a different type.
//
// - out / egress: originated from this host
// - in:  destination is this host.
// - ingress: terminate TLS, forward to a different host
// - relay: proxy based on SNI/metadata without change
func (ug *UGate) handleAcceptedConn(l *ugate.Listener, acceptedCon net.Conn) {
	// Attempt to determine the Listener and target
	// Ingress mode, forward to an IP
	cfg := l

	// Get a buffered connection with metadata from the pool.
	//c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

	tlsTerm := false

	// Get a buffered stream - this is used for sniffing.
	// Most common case is TLS, we want the SNI.
	bconn := ugate.GetConn(acceptedCon)
	ug.trackStreamIN(bconn.Meta())
	defer ug.onAcceptDone(bconn)
	str := bconn.Meta()

	// Special protocols, muxed on a single port - will extract real
	// destination and port.
	//
	// First are specific to egress capture.
	switch cfg.Protocol {
	case ugate.ProtoIPTablesIn:
		// iptables is replacing the conn - process before creating the buffer
		str.Dest, str.ReadErr = iptables.SniffIptables(str, cfg.Protocol)
		cfg = ug.findCfgIptablesIn(bconn)
	case ugate.ProtoIPTables:
		str.Dest, str.ReadErr = iptables.SniffIptables(str, cfg.Protocol)
		str.Egress = true
	case ugate.ProtoSocks:
		str.Egress = true
		str.ReadErr = socks.ReadSocksHeader(bconn)
	case ugate.ProtoHTTP:
		str.ReadErr = ug.h2Handler.handleHTTPListener(l, bconn)
		return
	case ugate.ProtoHTTPS:
		tlsTerm = true
		// Used to present the right cert
		str.ReadErr = sni.SniffSNI(bconn)
	case ugate.ProtoTLS:
		str.ReadErr = sni.SniffSNI(bconn)
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
		tc, err := ug.NewTLSConnIn(str.Context(), bconn, ug.TLSConfig)
		if err != nil {
			str.ReadErr = err
			log.Println("TLS: ", err)
			return
		}
		tlsOrOrigStr = tc.Meta()
	}

	str.ReadErr = ug.handleStream(tlsOrOrigStr)
}

// Auto-detect protocol on the wire, so routing info can be
// extracted:
// - TLS ( 22, 3)
// - HTTP
// - WS ( CONNECT )
// - SOCKS (5)
// - H2
// - TODO: HAproxy
//func sniff(pl *ugate.Listener, br *stream.RawConn) error {
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

var (
	// ClientPreface is the string that must be sent by new
	// connections from clients.
	h2ClientPreface = []byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")
)


func SniffH2(br *ugate.RawConn) error {
	br.Sniff()
	var proto string

	for {
		_, err := br.Fill()
		if err != nil {
			return err
		}
		off := br.End
		if ix := bytes.IndexByte(br.Buf[0:off], '\n'); ix >=0 {
			if bytes.Contains(br.Buf[0:off], []byte("HTTP/1.1")) {
				break
			}
		}
		if off >= len(h2ClientPreface) {
			if bytes.Equal(br.Buf[0:len(h2ClientPreface)], h2ClientPreface) {
				proto = ugate.ProtoH2
				break
			}
		}
	}

	// All bytes in the buffer will be Read again
	br.Reset(0)

	switch proto {
	case ugate.ProtoSocks:
		socks.ReadSocksHeader(br)
	case ugate.ProtoTLS:
		sni.SniffSNI(br)
	}
	br.Stream.Type = proto

	return nil
}
