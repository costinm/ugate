package dns

import (
	"log"
	"testing"

	"github.com/miekg/dns"
)

func TestLocal(t *testing.T) {
	s, _ := NewDmDns(5354)

	//h2, _ := transport.NewH2("")
	//h2.InitMTLSServer(5355, s)

	// client
	sc, err := NewDmDns(5356)
	if err != nil {
		t.Fatal(err)
	}
//	h2c, _ := transport.NewH2("")

//	sc.H2 = h2c.HttpsClient
//	sc.BaseUrl = "https://localhost:5355/dns?dns="

	s.AddRecord("a.dm.", dns.TypeA, &dns.A{
		Hdr: dns.RR_Header{Name: "a.dm.", Rrtype: dns.TypeA, Ttl: 100, Class: dns.ClassINET},
		A:   []byte{1, 2, 3, 4},
	})

	go s.Serve()

	q := dns.Question{Name: dns.Fqdn("home.webinf.info."), Qclass: dns.ClassINET, Qtype: dns.TypeA}
	m := new(dns.Msg)
	m.Id = dns.Id()
	m.RecursionDesired = true
	m.Question = []dns.Question{q}

	// Process a real request, using the process method
	t.Run("real", func(t *testing.T) {
		res := s.Process(m)
		log.Print(res)
	})

	t.Run("local", func(t *testing.T) {
		m = &dns.Msg{}
		m.SetQuestion("a.dm.", 1)
		res := s.Process(m)
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

	//t.Run("httpproxy", func(t *testing.T) {
	//
	//	res, err := sc.ForwardHttp(m)
	//	log.Print(res, err)
	//	//res, rtt, err := s.dnsUDPclient.Exchange(m, "localhost:5356")
	//	//log.Print(res, rtt, err)
	//
	//})

	t.Run("httpproxy1", func(t *testing.T) {
		sc.BaseUrl = "https://10.1.10.1:5228/dns?dns="
		res := sc.Process(m)
		log.Print(res)
		//res, rtt, err := s.dnsUDPclient.Exchange(m, "localhost:5356")
		//log.Print(res, rtt, err)

	})

	t.Run("httpproxy2", func(t *testing.T) {
		//sc.BaseUrl = "https://10.1.10.204:5228/dns?dns="
		sc.BaseUrl = "https://h.webinf.info:5228/dns?dns="
		res := sc.Process(m)
		log.Print(res)
		//res, rtt, err := s.dnsUDPclient.Exchange(m, "localhost:5356")
		//log.Print(res, rtt, err)

	})

	t.Run("ip6", func(t *testing.T) {
		sc.BaseUrl = "https://10.1.10.204:5228/dns?dns="
		q := dns.Question{Name: dns.Fqdn("www.google.com."), Qclass: dns.ClassINET, Qtype: dns.TypeAAAA}
		m := new(dns.Msg)
		m.Id = dns.Id()
		m.RecursionDesired = true
		m.Question = []dns.Question{q}

		//sc.BaseUrl = "https://h.webinf.info:5228/dns?dns="
		res := sc.Process(m)
		log.Print(res)
		//res, rtt, err := s.dnsUDPclient.Exchange(m, "localhost:5356")
		//log.Print(res, rtt, err)

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

// Not using mikedns for TCP, only UDP
