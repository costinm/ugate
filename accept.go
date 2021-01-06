package ugate

import (
	"bytes"
	"errors"
	"log"
	"net"
	"runtime/debug"
	"time"
)

// Called at the end of the connection handling. After this point
// nothing should use or refer to the connection, both proxy directions
// should already be closed for write or fully closed.
func (ug *UGate) onAcceptDone(rc MetaConn) {
	s := rc.Meta()
	ug.m.Lock()
	delete(ug.ActiveTcp, s.StreamId)
	ug.m.Unlock()
	tcpConActive.Add(-1)
	// TODO: track multiplexed streams separately.
	if s.ReadErr != nil {
		VarzSErrRead.Add(1)
	}
	if s.WriteErr != nil {
		VarzSErrWrite.Add(1)
	}
	if s.ProxyReadErr != nil {
		VarzCErrRead.Add(1)
	}
	if s.ProxyWriteErr != nil {
		VarzCErrWrite.Add(1)
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

	if s.ReadErr != nil || s.WriteErr != nil {
		log.Println("Err in:", s.ReadErr, s.WriteErr)
	}
	if s.ProxyReadErr != nil || s.ProxyWriteErr != nil {
		log.Println("Err out:", s.ProxyReadErr, s.ProxyWriteErr)
	}

	log.Printf("AC: %d src=%s://%v dst=%s rcv=%d/%d snd=%d/%d la=%v ra=%v op=%v",
		s.StreamId,
		s.Type, rc.RemoteAddr(),
		s.Request.Host,
		s.RcvdPackets, s.RcvdBytes,
		s.SentPackets, s.SentBytes,
		time.Since(s.LastWrite),
		time.Since(s.LastRead),
		int64(time.Since(s.Open).Seconds()))


	// Make sure it is closed.
	err := rc.Close()
	if err == nil {
		log.Println("Not closed ?")
	}
	if bc, ok := rc.(*RawConn); ok {
		bufferedConPool.Put(bc)
	}
}

// Deferred to connection close, only for the raw accepted connection.
func (ug *UGate) onAcceptDoneAndRecycle(rc *RawConn) {
	ug.onAcceptDone(rc)
	bufferedConPool.Put(rc)
}

// WIP: special handling for egress, i.e. locally originated streams.
// Identification:
// - dedicated listener ports for iptables, socks5 or tun
// - listeners with address 127.0.0.1
// - connections with src/dst address in 127.0.0.0/8
//
// The last 2 are 'whitebox' mode, using the port and address to select
// the routes.
//
// After determining the target from meta or config the request is proxied.
func (pl *PortListener) handleEgress(acceptedCon net.Conn) error {

	return nil
}

// WIP: handling for accepted connections for this node.
func (pl *PortListener) handleLocal(acceptedCon net.Conn) error {

	return nil
}

func (ug *UGate) 	HandleUdp(dstAddr net.IP, dstPort uint16, localAddr net.IP, localPort uint16, data []byte) {

}

// Hack for netstack/gvisor
// TODO: add it to the interface if not switching to lwip
var Reversed = false

// For BTS/H2 and iptables-in, the config for the actual listen port is virtual.
// LocalAddr port determines which config to use for routing.
// RemoteAddr is the (authenticated) remote VIP or the real client IP.
//
// TUN and iptables(out) are used for egress, remoteAddr is the destination on a remote
// computer, and localAddr is on same computer and not very useful.

// New style, based on lwip. Blocks until connect, proxy runs in background.
func (ug *UGate) HandleTUN(conn net.Conn, target *net.TCPAddr) error {
	bconn := GetConn(conn)
	bconn.Meta().Egress = true
	ug.trackStreamIN(bconn.Meta())
	defer ug.onAcceptDone(bconn)

	ra := conn.RemoteAddr()
	la := conn.LocalAddr()

	if Reversed {
		ta := ra
		ra = la
		la = ta
	}

	// Testing/debugging - localhost is captured by table local, rule 0.
	if bconn.Stream.Request.Host == "" {
		if ta, ok := ra.(*net.TCPAddr); ok {
			if ta.Port == 5201 {
				ta.IP = []byte{0x7f, 0, 0, 1}
			}
			bconn.Stream.Request.Host = ta.String()
			log.Println("LTUN ", conn.RemoteAddr(), conn.LocalAddr(), bconn.Stream.Request.Host)
		}
	}

	// TODO: config ? Could be shared with iptables port
	return ug.handleStream(bconn)
}

// Handle a virtual (multiplexed) stream, received over
// another connection.
func (ug *UGate) HandleVirtualIN(bconn MetaConn) error {
	ug.trackStreamIN(bconn.Meta())
	defer ug.onAcceptDone(bconn)

	if len(bconn.Meta().Request.Header) == 0 {
		// No metadata from headers, this is a plain stream.
		// Try to decode a request.

	}

	return ug.handleStream(bconn)
}

// At this point the stream has the metadata:
// - RequestURI
// - Host
// - Headers
// - TLS context
// The ListenerConfig may also be known.
func (ug *UGate) handleStream(bconn MetaConn) error {
	m := bconn.Meta()

	if m.Listener == nil {
		m.Listener = ug.findCfg(bconn)
	}
	cfg := m.Listener

	// Config has an in-process handler - not forwarding (or the handler may
	// forward).
	if cfg.Handler != nil {
		// SOCKS and others need to send something back - we don't
		// have a real connection, faking it.
		if pc, ok := bconn.(ProxyConn); ok {
			pc.PostDial(bconn, nil)
		}
		return cfg.Handler.Handle(bconn)
	}

	// By default, dial out
	return ug.dialOut(bconn, m.Listener)
}
func (ug *UGate) trackStreamIN(s *Stream) {
	ug.m.Lock()
	ug.ActiveTcp[s.StreamId] = s
	ug.m.Unlock()

	tcpConActive.Add(1)
	tcpConTotal.Add(1)
}

// A real accepted connection. Based on config, sniff and possibly
// dispatch to a different type.
//
// - out / egress: originated from this host
// - in:  destination is this host.
// - ingress: terminate TLS, forward to a different host
// - relay: proxy based on SNI/metadata without change
func (ug *UGate) handleAcceptedConn(pl *PortListener, acceptedCon net.Conn) error {
	var mConn MetaConn
	// Get a buffered connection with metadata from the pool.
	bconn := GetConn(acceptedCon)
	ug.trackStreamIN(bconn.Meta())
	defer pl.Gate.onAcceptDone(bconn)

	mConn = bconn

	// Attempt to determine the ListenerConf and target
	// Ingress mode, forward to an IP
	cfg := pl.cfg
	//c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))


	// Special listeners - will extract real destination and port
	// Both are also specific to egress.
	if cfg.Protocol == ProtoSocks {
		pl.Gate.sniffSOCKSConn(bconn)
		bconn.Meta().Egress = true
		cfg = ug.findCfg(bconn)
	} else if cfg.Protocol == "iptables" {
		bconn.Meta().Egress = true
		if _, ok := bconn.ServerOut.(*net.TCPConn); ok {
			pl.Gate.sniffIptables(bconn, cfg.Protocol)
		} else {
			return nil
		}

		cfg = ug.findCfg(bconn)
	// 	Special listener for multiplexed in or forwarding.
	} else if cfg.Protocol == "iptables-in" {
		if _, ok := bconn.ServerOut.(*net.TCPConn); ok {
			// This is typically for in, not mux (it's on a random port)
			pl.Gate.sniffIptables(bconn, cfg.Protocol)
		} else {
			return nil
		}
		cfg = ug.findCfg(bconn)
	} else if cfg.Protocol == ProtoTLS {
		// try to find the virtual host -
		bconn.Sniff()
		err := pl.Gate.sniffSNI(bconn)
		if err != nil {
			bconn.Stream.ReadErr = err
			return err
		}

		// More specific config - else use the default config on the tls
		l := ug.Conf[bconn.Meta().Request.Host]
		if l != nil {
			cfg = l
		}
	}

	if cfg == nil {
		cfg = pl.Gate.DefaultListener
	}

	// Virtual listeners should update the cfg based on Host

	// At this point we should have a 'cfg' for the virtual listener.
	bconn.Stream.Listener = cfg
	bconn.Meta().Type = cfg.Protocol

	if cfg.Remote != "" {
		bconn.Meta().Request.Host = cfg.Remote
	} else if cfg.Dialer != nil {
	} else if cfg.Protocol == "tcp" {
		// Nothing to do, plain text
	} else if cfg.Protocol == "http" {
		// Not explicitly configured. Detect TLS, HTTP, HTTP/2
		// TODO: HA-PROXY
		// socks is generally used on localhost.
		err := pl.sniff(bconn)
		if err != nil {
			return err
		}
	}

	// Terminate TLS if the stream is detected as TLS and the matched config
	// is configured for termination.
	// Else it's just a proxied SNI connection.
	if cfg.TLSConfig != nil {
		tc, err := NewTLSConnIn(bconn.Meta().Request.Context(), bconn, cfg.TLSConfig)
		if err != nil {
			return err
		}
		mConn = tc
	}

	return ug.handleStream(mConn)
}

// Auto-detect protocol on the wire, so routing info can be
// extracted:
// - TLS ( 22, 3)
// - HTTP
// - WS ( CONNECT )
// - SOCKS (5)
// - H2
// - TODO: HAproxy
func (pl *PortListener) sniff(br *RawConn) error {
	br.Sniff()
	var proto string

	for {
		_, err := br.Fill()
		if err != nil {
			return err
		}
		off := br.end
		if off >= 2 {
			b0 := br.buf[0]
			b1 := br.buf[1]
			if b0 == 5 {
				proto = ProtoSocks
				break;
			}
			// TLS or SNI - based on the hostname we may terminate locally !
			if b0 == 22 && b1 == 3 {
				// 22 03 01..03
				proto = ProtoTLS
				break
			}
		}
		if off >= 7 {
			if bytes.Equal(br.buf[0:7], []byte("CONNECT")) {
				proto = ProtoConnect
				break
			}
		}
		if off >= len(h2ClientPreface) {
			if bytes.Equal(br.buf[0:len(h2ClientPreface)], h2ClientPreface) {
				proto = ProtoH2
				break
			}
		}
	}

	// All bytes in the buffer will be Read again
	br.Reset(0)

	switch proto {
	case ProtoSocks:
		pl.Gate.sniffSOCKSConn(br)
	case ProtoTLS:
		pl.Gate.sniffSNI(br)
	}
	br.Stream.Type = proto

	return nil
}

