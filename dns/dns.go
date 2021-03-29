package dns

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// TODO: option to use local resolvers instead of 8.8.8.8
// TODO: customize the upstream (real) DNS
// TODO: if it can't connect, return fake IP to simulate walled garden wifi

// http://mkaczanowski.com/golang-build-dynamic-dns-service-go/

var (
	// createBuffer to get a buffer. Inspired from caddy.
	// See PooledIOCopy for example
	bufferPoolCopy = sync.Pool{New: func() interface{} {
		return make([]byte, 0, 32*1024)
	}}

	bufferPoolUdp = sync.Pool{New: func() interface{} {
		return make([]byte, 0, 1600)
	}}

	bufferPoolHttp = sync.Pool{New: func() interface{} {
		return make([]byte, 0, 4096)
	}}
)

// UdpWriter is the interface implemented by the TunTransport, to send
// packets back to the virtual interface
// Set by TProxy and TUN capture. If missing, a regular UDP will be used,
// first with WriteMsgUdp and if it fails without preserving srcAddr.
type UdpWriter interface {
	WriteTo(data []byte, dstAddr *net.UDPAddr, srcAddr *net.UDPAddr) (int, error)
}


type DmDns struct {
	// used by the dns proxy for forwarding queries to real DNS/UDP clients
	dnsUDPclient *dns.Client

	UDPConn *net.UDPConn

	// UDP
	// Capture return - sends packets back to client app.
	// This is typically a netstack or TProxy
	UDPWriter UdpWriter

	// Client used for communicating with the gateway - should be capable of H2, and have
	// all authetication set up.
	H2 *http.Client

	// Address and port for the DNS-over-https gateway. If empty, direct calls
	// using dnsUDPClient.
	BaseUrl string

	// Local DNS server, respond with local entries or forwards.
	dnsServer *dns.Server

	dnsLock sync.RWMutex
	// TODO: periodic cleanup by ts
	dnsByAddr map[string]*DnsEntry

	dnsByName map[string]*DnsEntry

	// local DNS entries. Both server and 'client'
	dnsEntries map[string]map[uint16]dns.RR

	// Nameservers to use for direct calls, without a VPN.
	// Overriden from "DNS" env variable.
	nameservers []string
	Port        int
}

// Info and stats about a DNS entry.
type DnsEntry struct {
	// Last time it was returned.
	ts time.Time

	// DNS name, with trailing .
	Name string

	IP net.IP

	// Number of times it was called.
	Count int

	RCount int

	// Latency on getting the entry
	Lat time.Duration
}

// Blocking
func (s *DmDns) Serve() {
	s.dnsServer.ActivateAndServe()
}

func (s *DmDns) Start(mux *http.ServeMux) {
	net.DefaultResolver.PreferGo = true
	net.DefaultResolver.Dial = DNSDialer(s.Port)
	go s.Serve()
}

// Given an IPv4 or IPv6 address, return the name if DNS was used.
func (s *DmDns) NameByAddr(addr string) (*DnsEntry, bool) {
	if s == nil {
		return nil, false
	}
	s.dnsLock.RLock()
	dns, f := s.dnsByAddr[addr]
	s.dnsLock.RUnlock()
	return dns, f
}

// HostByAddr returns the last lookup address for an IP, or the original
// address. The IP is expressed as a string ( ip.String() ).
func (s *DmDns) HostByAddr(addr string) (string, bool) {
	e, ok := s.NameByAddr(addr)
	if ok {
		return e.Name, ok
	}
	return addr, ok
}

func (s *DmDns) IPResolve(ip string) string {
	de, f := s.NameByAddr(ip)
	if !f {
		return ""
	}
	return de.Name
}

// New DNS server, listening on port.
func NewDmDns(port int) (*DmDns, error) {
	d := &DmDns{
		Port: port,
		dnsUDPclient: &dns.Client{},
		dnsEntries:   map[string]map[uint16]dns.RR{},
		dnsByAddr:    make(map[string]*DnsEntry),
		dnsByName:    make(map[string]*DnsEntry),
		nameservers: []string{
			"1.1.1.1:53", // cloudflare
			//"209.244.0.3:53", // level3
			//"74.82.42.42:53", // HE
			//"8.8.8.8:53",     //google
			//"208.67.222.222:53", // opendns
		},
	}

	dnsOverride := os.Getenv("DNS")
	if dnsOverride != "" {
		d.nameservers = strings.Split(dnsOverride, ",")
	}

	if port != -1 {
		addr := ":" + strconv.Itoa(port)
		d.dnsServer = &dns.Server{
			Addr:         addr,
			Net:          "udp",
			WriteTimeout: 3 * time.Second,
			ReadTimeout:  15 * time.Minute}

		dns.HandleFunc(".", func(w dns.ResponseWriter, req *dns.Msg) {
				m := d.Process(req)
			writeMsg(w, m)
		})

		a, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return d, err
		}
		l, err := net.ListenUDP("udp", a)
		if err != nil {
			return d, err
		}
		log.Println("Starting DNS server ", port)
		d.UDPConn = l
		err6 := ipv6.NewPacketConn(l).SetControlMessage(ipv6.FlagDst|ipv6.FlagInterface, true)
		err4 := ipv4.NewPacketConn(l).SetControlMessage(ipv4.FlagDst|ipv4.FlagInterface, true)
		if err6 != nil && err4 != nil {
			return d, err4
		}
		d.dnsServer.PacketConn = l

		go d.dnsServer.ActivateAndServe()

		// TODO: also on TCP

	}
	return d, nil
}

// DNSOverTCP implements DNS over TCP protocol. Used in TCP capture, for port 53.
// TODO: also as a standalone server.
func (s *DmDns) DNSOverTCP(in io.ReadCloser, out io.Writer) error {
	pbuf := bufferPoolCopy.Get().([]byte)
	defer bufferPoolCopy.Put(pbuf)
	bufCap := cap(pbuf)
	buf := pbuf[0:bufCap:bufCap]

	var err error
	var er error

	// data between 0 and off
	off := 0

	// incomplete packet - needs to fill the buffer
	needRead := true

	// number read in last Read operation
	nr := 0

	resB := bufferPoolHttp.Get().([]byte)
	defer bufferPoolHttp.Put(resB)
	resB = resB[0:4096:4096]
	nreq := 0

	defer log.Print("DNS TCP Close", nreq, err)

	for {
		if needRead {
			nr, er = in.Read(buf[off:])
			if er != nil || nr <= 0 {
				return er
			}
		} else {
			nr = off
			needRead = true
		}
		end := off + nr
		if end < 2 {
			off += nr
			//log.Println("TCPDNS: Short read, shouldn't happen ", nr, off)
			needRead = true
			continue
		}
		packetLen := int(buf[0])*256 + int(buf[1])
		if packetLen+2 < end {
			//log.Println("TCPDNS: Short packet read, shouldn't happen ", nr, off)
			off += nr
			needRead = true
			continue
		}

		req := new(dns.Msg)
		err = req.Unpack(buf[2 : 2+packetLen])

		// TODO: in a go routine, to not block
		res := s.Process(req)

		resBB, _ := res.PackBuffer(resB[2:])
		binary.BigEndian.PutUint16(resB[0:], uint16(len(resBB)))
		nw, ew := out.Write(resB[0 : len(resBB)+2])

		nreq++

		if ew != nil {
			return ew
		}
		if nr != nw {
			return io.ErrShortWrite
		}

		if 2+packetLen < end {
			copy(buf[0:], buf[2+packetLen:end])
			off = end - 2 - packetLen
			end = off
			needRead = false
			continue
		}
	}

	return err
}

// Process resolves a query by forwarding to a recursive nameserver or handling it locally.
// This is the main function - can be called from:
// - the real local UDP DNS (mike's)
// - DNS-over-TCP or TLS server
// - captured UDP:53 from TUN
//
// Wrapps the real process method with stats gathering and builds a reverse map of IP to names
func (s *DmDns) Process(req *dns.Msg) *dns.Msg {
	//ClientMetrics.Total.Add(1)
	if len(req.Question) == 0 {
		m := new(dns.Msg)
		m.SetRcode(req, dns.RcodeServerFailure)
		//ClientMetrics.Errors.Add(1)
		return m
	}

	parts := strings.Split(req.Question[0].Name, ".")

	// Android/etc probes for DNS server - blahblah.
	if len(parts) == 0 || len(parts) <= 2 && len(parts[0]) > 5 {
		m := new(dns.Msg)
		m.SetRcode(req, dns.RcodeNameError)
		return m
	}
	t0 := time.Now()
	s.dnsLock.Lock()

	for _, a := range req.Question {
		if a.Name == "" {
			continue
		}
		if a.Qtype == dns.TypeA || a.Qtype == dns.TypeAAAA {
			entry, f := s.dnsByName[a.Name]
			if !f {
				entry = &DnsEntry{
					Name: a.Name,
				}
				s.dnsByName[a.Name] = entry
			}
			entry.Count++
			entry.ts = time.Now()
		}
	}
	s.dnsLock.Unlock()

	res := s.process(req)

	if len(res.Answer) > 0 {
		d := time.Since(t0)
		if d > 1*time.Second {
			log.Println("DNS: ", req.Question[0].Name, d, res.Answer)
		}
		//ClientMetrics.Latency.Add(time.Since(t0).Seconds())
	}
	if res == nil {
		m := new(dns.Msg)
		m.SetRcode(req, dns.RcodeServerFailure)
		//ClientMetrics.Errors.Add(1)
		return res
	}

	s.dnsLock.Lock()
	for cnt, a := range res.Answer {
		e, _ := s.dnsByName[res.Answer[cnt].Header().Name]
		if e == nil {
			continue
		}
		if a.Header().Rrtype == dns.TypeA {
			ip := a.(*dns.A).A
			e.IP = ip
			entry, f := s.dnsByAddr[ip.String()]
			if !f {
				entry = e
				s.dnsByAddr[ip.String()] = entry
			}
			entry.RCount++
			entry.Lat = time.Now().Sub(e.ts)
		} else if a.Header().Rrtype == dns.TypeAAAA {
			ip := a.(*dns.AAAA).AAAA
			e.IP = ip
			entry, f := s.dnsByAddr[ip.String()]
			if !f {
				entry = e
				s.dnsByAddr[ip.String()] = entry
			}
			entry.RCount++
			entry.Lat = time.Now().Sub(e.ts)
		}
	}
	s.dnsLock.Unlock()

	return res
}

// Actual processing of the request (wrapped in Process with stats)
func (s *DmDns) process(req *dns.Msg) *dns.Msg {
	name := req.Question[0].Name

	if strings.HasSuffix(name, ".dm.") {
		m := new(dns.Msg)
		m.SetReply(req)
		m.Compress = false

		switch req.Opcode {
		case dns.OpcodeQuery:
			s.localQuery(m)
		}
		return m
	}
	if strings.HasSuffix(name, ".m.") {
		m := new(dns.Msg)
		m.SetReply(req)
		m.Compress = false

		switch req.Opcode {
		case dns.OpcodeQuery:
			// TODO: resolve to 'next hop' in the mesh
			s.localQuery(m)
		}
		return m
	}

	var res *dns.Msg
	var err error

	//if s.BaseUrl != "" && !strings.Contains(s.BaseUrl, name[0:len(name)-1]) {
	//	res, err = s.ForwardHttp(req)
	//} else {

	// TODO: use upstream control message for DNS
	res, err = s.ForwardRealDNS(req)
	//	}

	if err == nil {
		res.Compress = true
		return res
	}

	//log.Printf("DNSE: '%s' %v", name, err)
	//ClientMetrics.Errors.Add(1)

	m := new(dns.Msg)
	m.SetRcode(req, dns.RcodeServerFailure)
	return m
}

const dnsTest = false

type dnsRes struct {
	msg *dns.Msg
	s   string
	err error
}

// ForwardRealDNS sends the query to real nameservers.
func (s *DmDns) ForwardRealDNS(req *dns.Msg) (*dns.Msg, error) {
	var nsIdx int
	var r *dns.Msg
	var err error

	nservers := s.nameservers

	// Send to all nameservers, find the fastest
	if dnsTest {
		t0 := time.Now()
		res := make(chan dnsRes, 10)

		for _, ns := range nservers {
			ns := ns
			go func() {
				r, _, err = s.dnsUDPclient.Exchange(req, ns)
				//log.Println("Got res ", ns, time.Since(t0))
				res <- dnsRes{msg: r, s: ns, err: err}
			}()

		}
		rr := <-res
		r := rr.msg

		if rr.err == nil {
			switch r.Rcode {
			// SUCCESS
			case dns.RcodeSuccess:
				fallthrough
			case dns.RcodeNameError:
				fallthrough
				// NO RECOVERY
			case dns.RcodeFormatError:
				fallthrough
			case dns.RcodeRefused:
				fallthrough
			case dns.RcodeNotImplemented:
				log.Println("DNS", time.Since(t0), rr.s)
				return r, err
			}
		}
		return r, rr.err
	}

	for try := 1; try <= 2; try++ {

		r, _, err = s.dnsUDPclient.Exchange(req, nservers[nsIdx])

		if err == nil {
			switch r.Rcode {
			// SUCCESS
			case dns.RcodeSuccess:
				fallthrough
			case dns.RcodeNameError:
				fallthrough
				// NO RECOVERY
			case dns.RcodeFormatError:
				fallthrough
			case dns.RcodeRefused:
				fallthrough
			case dns.RcodeNotImplemented:
				return r, err
			}
		}

		if err != nil {
			//log.Println("DNS err:", req.Id, nservers[nsIdx], req.Question[0].Name, err)
		}

		// Continue with next available DmDns
		if len(nservers)-1 > nsIdx {
			nsIdx++
		} else {
			nsIdx = 0
			break
		}
	}

	return r, err
}

func writeMsg(w dns.ResponseWriter, m *dns.Msg) {
	if err := w.WriteMsg(m); err != nil {
		log.Printf("[%d] Failed to return reply: %v", m.Id, err)
	}
}

// Local request handling, for .dm. virtual domain
//

func (s *DmDns) getRecord(domain string, rtype uint16) (rr dns.RR, found bool) {
	s.dnsLock.RLock()
	defer s.dnsLock.RUnlock()
	rrmap, found := s.dnsEntries[domain]
	if !found {
		return
	}
	rr, found = rrmap[rtype]
	return
}

func (s *DmDns) AddRecord(domain string, rtype uint16, rr dns.RR) {
	s.dnsLock.RLock()
	defer s.dnsLock.RUnlock()
	rrmap, found := s.dnsEntries[domain]
	if !found {
		rrmap = map[uint16]dns.RR{}
	}
	rrmap[rtype] = rr
	return
}

// Called for queries matching the authoritative domains.
func (s *DmDns) localQuery(m *dns.Msg) bool {
	var rr dns.RR

	needsFwd := false
	for _, q := range m.Question {
		if read_rr, e := s.getRecord(q.Name, q.Qtype); e {
			rr = read_rr.(dns.RR)
			m.Answer = append(m.Answer, rr)
		} else {
			// No explicit override.
		}
	}
	return needsFwd
}

func DNSDialer(port int) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{}
		//return d.DialContext(ctx, "udp", "1.1.1.1:53")
		return d.DialContext(ctx, "udp", fmt.Sprintf("127.0.0.1:%d", port))
	}
}

// HttpDebugDNS dumps DNS cache (dnsByName)
func (s *DmDns) HttpDebugDNS(w http.ResponseWriter, r *http.Request) {
	s.dnsLock.RLock()
	defer s.dnsLock.RUnlock()
	w.Header().Add("Content-type", "text/plain")
	for _, d := range s.dnsByName {
		if d.IP != nil {
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%v\n", d.IP.String(), d.Name, d.Count, d.RCount, d.Lat)
		} else {
			fmt.Fprintf(w, "-\t%s\t%d\n", d.Name, d.Count)
		}
	}
	//json.NewEncoder(w).Encode(p.DnsByAddr)
}

// Special capture for DNS. Will use the DNS VPN or direct calls.
func (gw *DmDns) HandleUdp(dstAddr net.IP, dstPort uint16, localAddr net.IP, localPort uint16, data []byte) {
	req := new(dns.Msg)
	req.Unpack(data)

	res := gw.Process(req)

	data1, _ := res.Pack()
	src := &net.UDPAddr{Port: int(localPort), IP: localAddr}

	if gw.UDPWriter != nil {
		srcAddr := &net.UDPAddr{IP: dstAddr, Port: int(dstPort)}
		n, err := gw.UDPWriter.WriteTo(data1, src, srcAddr)
		if err != nil {
			log.Print("Failed to send udp dns ", err, n)
		}
	} else {
		// Attempt to write as UDP
		cm4 := new(ipv4.ControlMessage)
		cm4.Src = dstAddr
		oob := cm4.Marshal()
		_, _, err := gw.UDPConn.WriteMsgUDP(data1, oob, src)
		if err != nil {
			_, err = gw.UDPConn.WriteToUDP(data1, src)
			if err != nil {
				log.Print("Failed to send DNS ", dstAddr, dstPort, src)
			}
		}
	}
}
