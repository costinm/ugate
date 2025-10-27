package dns

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/miekg/dns"
)

func TestLocal(t *testing.T) {
	ctx := context.Background()

	s := New()
	s.Port = 5354
	err := s.Provision((ctx))
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// client
	sc := New()
	sc.Port = 5356
	sc.Capture = true
	defer func() {
		net.DefaultResolver.PreferGo = false
		net.DefaultResolver.Dial = nil
	}()
	err = sc.Provision((ctx))
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	s.AddRecord("a.dm.", dns.TypeA,
		&dns.A{
			Hdr: dns.RR_Header{Name: "a.dm.", Rrtype: dns.TypeA, Ttl: 100, Class: dns.ClassINET},
			A:   []byte{1, 2, 3, 4},
		})

	err = s.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	q := dns.Question{Name: dns.Fqdn("home.webinf.info."), Qclass: dns.ClassINET, Qtype: dns.TypeA}

	m := new(dns.Msg)
	m.Id = dns.Id()
	m.RecursionDesired = true
	m.Question = []dns.Question{q}

	t.Run("netdns", func(t *testing.T) {
		// Has trace support (if ctx has nettrace.TraceKey{})
		// There is a hook to allow replacing the function for 'tests' - the
		// override is the LookupIP - the one return []IPAddr
		meshr := &net.Resolver{
			PreferGo: true,
			Dial:     DNSDialer(s.Port),
		}
		// addrs, err := net.LookupHost("a.dm.")
		// t.Log(addrs)
		// if err != nil {
		// 	t.Fatalf("LookupHost failed: %v", err)
		// }
		addrs, err := meshr.LookupHost(ctx, "a.dm.")
		t.Log(addrs)
		if err != nil {
			t.Fatalf("LookupHost failed: %v", err)
		}

		meshr.LookupIPAddr(ctx, "10.0.0.1")
		meshr.LookupIP(ctx, "ip6", "10.0.0.1")

	})

	t.Run("local", func(t *testing.T) {
		m = &dns.Msg{}
		m.SetQuestion("a.dm.", 1)
		res := s.Do(m)
		log.Print(res)
	})

	t.Run("proxy", func(t *testing.T) {
		res, rtt, err := s.dnsUDPclient.Exchange(m, "localhost:5354")

		log.Print(res, rtt, err)
	})

	t.Run("proxy6", func(t *testing.T) {
		res, rtt, err := s.dnsUDPclient.Exchange(m, "[2001:470:1f04:429:80::4]:53")

		log.Print(res, rtt, err)
	})

	// Life of a message:
	// - server.serveUdp() - readUDP
	// - server.serve(addr, h, byte, udpcon,..)
	// - req = new Msg; req.Unpack(data)
	// - handler, with response writer
	// - WriteToSessionUDP
	// - conn.WriteMsgUDP(b, oob, session.raddr)
	//
	// FlagDst and FlagInterface are set on the PacketConn
}
