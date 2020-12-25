package ugate

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net"
	"net/http/httptrace"
	"runtime/debug"
	"time"
)

// Deferred to connection close.
func (pl *PortListener) onDone(rc *RawConn) {
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
	log.Printf("AC: %d src=%s://%v dst=%s rcv=%d/%d snd=%d/%d la=%v ra=%v op=%v",
		rc.Stats.StreamId,
		rc.Stats.Type, rc.RemoteAddr(),
		rc.Meta().Target,
		rc.Stats.ReadPackets, rc.Stats.ReadBytes,
		rc.Stats.WritePackets, rc.Stats.WriteBytes,
		time.Since(rc.Stats.LastWrite),
		time.Since(rc.Stats.LastRead),
		time.Since(rc.Stats.Open))

	rc.Close()

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

// A stream can be:
// - out / egress: originated from this host
// - in:  destination is this host.
// - ingress: terminate TLS, forward to a different host
// - relay: proxy based on SNI/metadata without change
func (pl *PortListener) handleAcceptedConn(acceptedCon net.Conn) error {
	// c is the local or 'client' connection in this case.
	// 'remote' is the configured destination.

	//c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

	//ra := rc.RemoteAddr().(*net.TCPAddr)

	// 1. Try to match a Listener based on remote addr/port and local addr/port

	// 2. If we don't have enough info, sniff.
	// Client first protocols can't support multiplexing, must have single config

	// TODO: poll the readers
	bconn := GetConn(acceptedCon)
	defer pl.onDone(bconn)
	bconn.Stats.Accepted = true

	var mConn MetaConn
	mConn = bconn

	// Attempt to determine the ListenerConf and target
	// Ingress mode, forward to an IP
	cfg := pl.cfg

	switch cfg.Protocol {
	case ProtoSocks:

	}

	bconn.Stats.Type = cfg.Protocol
	if cfg.Remote != "" {
		bconn.Meta().Target = cfg.Remote
	} else if cfg.Dialer != nil {
	} else if cfg.Protocol == ProtoSocks {
		pl.GW.sniffSOCKSConn(bconn)
	} else if cfg.Protocol == ProtoTLS {
		pl.GW.sniffSNI(bconn)
	} else if cfg.Protocol == "iptables" ||
			cfg.Protocol == "iptables-in" {
		if _, ok := bconn.raw.(*net.TCPConn); ok {
			pl.GW.sniffIptables(bconn, cfg.Protocol)
		} else {
			return nil
		}
	} else if cfg.Protocol == "tcp" {
		// Nothing to do, plain text
	} else { // "mux"
		// Not explicitly configured. Detect TLS, HTTP, HTTP/2
		// TODO: HA-PROXY
		// socks is generally used on localhost.
		err := pl.sniff(bconn)
		if err != nil {
			return err
		}
	}

	// sets clientEventContextKey - if ctx is used for a round trip, will
	// set all data.
	// Will also make sure DNSStart, Connect, etc are set (if we want to)
	ctx, cancel := context.WithCancel(context.Background())
	ctx = httptrace.WithClientTrace(ctx, &bconn.Stats.ClientTrace)
	ctx = context.WithValue(ctx, "ugate.conn", bconn)
	bconn.Stats.Context = ctx
	bconn.Stats.ContextCancel = cancel

	if bconn.Stats.Type == "tls" && cfg.TLSConfig != nil {
		tc, err := NewTLSConnIn(ctx, bconn, cfg.TLSConfig)
		if err != nil {
			return err
		}
		mConn = tc
	}

	if cfg.Handler != nil {
		// SOCKS and others need to send something back - we don't
		// have a real connection, faking it.
		if bconn.postDial != nil {
			bconn.postDial(bconn, nil)
		}
		return cfg.Handler.Handle(mConn)
	}

	err := pl.dialOut(ctx, mConn, cfg)

	// The dialed connection has stats, so does the accept connection.

	return err
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
		err := br.Fill()
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
		pl.GW.sniffSOCKSConn(br)
	case ProtoTLS:
		pl.GW.sniffSNI(br)
	}
	br.Stats.Type = proto

	return nil
}

