package ugate

import (
	"bytes"
	"context"
	"log"
	"net"
	"net/http/httptrace"
	"time"
)

func (ll *portListener) onDone(rc *BufferedConn) {
	if rc.Stats.ReadErr != nil {
		VarzSErrRead.Add(1)
	}
	if rc.Stats.WriteErr != nil {
		VarzSErrWrite.Add(1)
	}
	if rc.Stats.ProxyReadErr != nil {
		VarzCErrRead.Add(1)
	}
	if rc.Stats.ProxyWriteErr != nil {
		VarzCErrWrite.Add(1)
	}
	log.Printf("A: %d src=%s://%v dst=%s rcv=%d/%d snd=%d/%d la=%v ra=%v op=%v",
		rc.Stats.StreamId,
		rc.Stats.Type, rc.RemoteAddr(),
		rc.Target,
		rc.Stats.ReadPackets, rc.Stats.ReadBytes,
		rc.Stats.WritePackets, rc.Stats.WriteBytes,
		time.Since(rc.Stats.LastWrite),
		time.Since(rc.Stats.LastRead),
		time.Since(rc.Stats.Open))

	rc.Close()

	bufferedConPool.Put(rc)
}

func (ll *portListener) handleAcceptedConn(rc net.Conn) error {
	// c is the local or 'client' connection in this case.
	// 'remote' is the configured destination.

	//c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

	//ra := rc.RemoteAddr().(*net.TCPAddr)

	// 1. Try to match a Listener based on remote addr/port and local addr/port

	// 2. If we don't have enough info, sniff.
	// Client first protocols can't support multiplexing, must have single config

	// TODO: poll the readers
	br := GetConn(rc)
	defer ll.onDone(br)

	// Attempt to determine the ListenerConf and target
	// Ingress mode, forward to an IP
	cfg := ll.cfg

	if cfg.Remote != "" {
		br.Target = cfg.Remote
	} else if cfg.Dialer != nil {
	} else if cfg.Protocol == "socks5" {
		ll.GW.serveSOCKSConn(br)
	} else if cfg.Protocol == "sni" {
		ll.GW.serveConnSni(br)
	} else if cfg.Protocol == "iptables" ||
			cfg.Protocol == "iptables-in" {
		if _, ok := br.Conn.(*net.TCPConn); ok {
			ll.GW.iptablesServeConn(br, cfg.Protocol)
		} else {
			return nil
		}
	} else {
		// Not explicitly configured. Detect TLS, HTTP, HTTP/2
		// TODO: HA-PROXY
		// socks is generally used on localhost.
		err := ll.sniff(br)
		if err != nil {
			return err
		}
	}

	// sets clientEventContextKey - if ctx is used for a round trip, will
	// set all data.
	// Will also make sure DNSStart, Connect, etc are set (if we want to)
	ctx := httptrace.WithClientTrace(context.Background(), &br.Stats.ClientTrace)

	if cfg.Handler != nil {
		if br.postDial != nil {
			// SOCKS and others need to send something back - we don't
			// have a real connection, faking it.
			br.postDial(br, nil)
		}
		return cfg.Handler.Handle(br)
	}

	var nc net.Conn
	var err error
	// SSH or in-process connectors
	if cfg.Dialer != nil {
		nc, err = cfg.Dialer.DialContext(ctx, "tcp", br.Target)
		if err != nil {
			return err
		}
	} else {
		nc, err = ll.GW.Dialer.DialContext(ctx, "tcp", br.Target)
	}

	if br.postDial != nil {
		br.postDial(nc, err)
	}
	if err != nil {
		log.Println("Failed to connect ", cfg.Remote, err)
		return err
	}

	err = br.Proxy(nc)

	// The dialed connection has stats, so does the accept connection.

	return err
}

// Auto-detect protocol on the wire, so routing info can be
// extracted:
// - TLS
// - HTTP
// - WS
// - H2
// - TODO: HAproxy
func (ll *portListener) sniff(br *BufferedConn) error {
	br.Sniff()
	var proto string

	off := 0
	for {
		n, err := br.Read(br.buf[off:])
		if err != nil {
			return err
		}
		off += n
		if off >= 2 {
			b0 := br.buf[0]
			b1 := br.buf[1]
			if b0 == 5 {
				proto = "socks5"
				break;
			}
			// TLS or SNI - based on the hostname we may terminate locally !
			if b0 == 22 && b1 == 3 {
				// 22 03 01..03
				proto = "sni"
				break
			}
			// TODO: CONNECT, WS else try HTTP/1.1 or HTTP/2 or gRPC
		}
		if off >= 7 {
			if bytes.Equal(br.buf[0:7], []byte("CONNECT")) {
				proto = "ws"
				break
			}
		}
		if off >= len(h2ClientPreface) {
			if bytes.Equal(br.buf[0:len(h2ClientPreface)], h2ClientPreface) {
				proto = "h2"
				break
			}
		}
	}

	// All bytes in the buffer will be Read again
	br.Reset(0)

	switch proto {
	case "socks5":
		ll.GW.serveSOCKSConn(br)
	case "sni":
		ll.GW.serveConnSni(br)
	}

	return nil
}

