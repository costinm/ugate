package dns

import (
	"context"
	"encoding/binary"
	"expvar"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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

	bufferPoolHttp = sync.Pool{New: func() interface{} {
		return make([]byte, 0, 4096)
	}}
)

func init() {
	// Implements: ServeHTTP - as handler supports DOH
	//
	expvar.Publish("modules.dns", expvar.Func(func() any { return New() }))
}

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

	// Client used for DNS-over-HTTPS requests.
	H2 *http.Client

	// Address and port for the DNS-over-https gateway. If empty, direct calls
	// using dnsUDPClient.
	BaseUrl string `json:"dohURL,omitempty"`

	// Local DNS server, respond with local entries or forwards.
	dnsServer *dns.Server

	dnsLock sync.RWMutex
	// TODO: periodic cleanup by ts
	dnsByAddr map[string]*DnsEntry

	// dnsByName - loaded during provision from config(s) or dynamically added.
	// Files and resource store can also hold on-demand records.
	dnsByName map[string]*DnsEntry

	Records map[string]*Record

	// local DNS entries. Both server and 'client'
	dnsEntries map[string]map[uint16]dns.RR

	// Nameservers to use for direct calls, without a VPN.
	// Overriden from "DNS" env variable.
	Nameservers []string
	Port        int

	Mux *http.ServeMux

	Capture bool
}

type Record map[string][]string

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

func New() *DmDns {
	return &DmDns{
		dnsUDPclient: &dns.Client{},
		dnsEntries:   map[string]map[uint16]dns.RR{},
		dnsByAddr:    make(map[string]*DnsEntry),
		dnsByName:    make(map[string]*DnsEntry),
	}
}

func (d *DmDns) Provision(ctx context.Context) error {
	if d.Nameservers == nil {
		d.Nameservers = []string{
			"1.1.1.1:53", // cloudflare
			//"209.244.0.3:53", // level3
			//"74.82.42.42:53", // HE
			//"8.8.8.8:53",     //google
			//"208.67.222.222:53", // opendns
		}
	}
	port := d.Port
	if port == 0 {
		port = 15053
	}

	addr := ":" + strconv.Itoa(port)
	d.dnsServer = &dns.Server{
		Addr:         addr,
		Net:          "udp",
		WriteTimeout: 3 * time.Second,
		ReadTimeout:  15 * time.Minute}

	// Can also set Handler instead of using ServerMux
	dns.HandleFunc(".", func(w dns.ResponseWriter, req *dns.Msg) {
		m := d.Do(req)
		writeMsg(w, m)
	})

	a, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	l, err := net.ListenUDP("udp", a)
	if err != nil {
		return err
	}

	d.UDPConn = l
	err6 := ipv6.NewPacketConn(l).SetControlMessage(ipv6.FlagDst|ipv6.FlagInterface, true)
	err4 := ipv4.NewPacketConn(l).SetControlMessage(ipv4.FlagDst|ipv4.FlagInterface, true)
	if err6 != nil && err4 != nil {
		return err4
	}
	d.dnsServer.PacketConn = l

	// if d.Mux != nil {
	// 	d.Mux.Handle("/dns/", d)
	// }

	// TODO: also on TCP

	return nil
}

func (s *DmDns) Start(ctx context.Context) error {
	// MAYBE: separate dns client ?
	// Base is x/net/dns - but without generic lookups.
	// Root is /etc/resolv.conf

	// Replacing the golang DNS resolver can be useful - but I think it increases
	// complexity and risks. Some code will not be go and use the /etc/resolv.conf
	// directly or via some other library.

	// Best option remains to set it to a per-host resolver on :53, with each VM
	// on the host pointing to it. Interception is not such a good idea, but works
	// as a backup (UDP and TCP).
	if s.Capture {
		// For Dial(), use this resolver.
		net.DefaultResolver.PreferGo = true
		// Install a dialer that connects to DmDns as resolver.
		// If it implements PacketConn, UDP will be used.
		// Otherwise it is used as DOT.
		//
		net.DefaultResolver.Dial = DNSDialer(s.Port)
	}

	if s.Port > 0 {
		// Will use PacketConn if not nil - and set the socket options.
		// Will also serve on the Listener as DOT.
		go s.dnsServer.ActivateAndServe()
	}
	return nil
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
		res := s.Do(req)

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

/*

CoreDNS:
- plugin.Register("name", func(caddy.Controller)error)) // forked caddy
- needs a Name() method
- plugin.Handler - ServeDNS takes a dns.ResponseWriter

*/

func (s *DmDns) ServeDNS(req *dns.Msg, w dns.ResponseWriter) (int, error) {
	res := s.Do(req)
	err := w.WriteMsg(res)
	return 0, err
}

// Do resolves a query by forwarding to a recursive nameserver or handling it locally.
// This is the main function - can be called from:
// - the real local UDP DNS (mike's)
// - DNS-over-TCP or TLS server
// - captured UDP:53 from TUN
//
// Wrapps the real process method with stats gathering and builds a reverse map of IP to names
func (s *DmDns) Do(req *dns.Msg) *dns.Msg {
	//ClientMetrics.Total.StartListener(1)
	if len(req.Question) == 0 {
		m := new(dns.Msg)
		m.SetRcode(req, dns.RcodeServerFailure)
		//ClientMetrics.Errors.StartListener(1)
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
		//ClientMetrics.Latency.StartListener(time.Since(t0).Seconds())
	}
	if res == nil {
		m := new(dns.Msg)
		m.SetRcode(req, dns.RcodeServerFailure)
		//ClientMetrics.Errors.StartListener(1)
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
	//ClientMetrics.Errors.StartListener(1)

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

	nservers := s.Nameservers

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
			//log.Println("DNS err:", req.muxID, nservers[nsIdx], req.Question[0].Name, err)
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

// DNSDialer will return a dialer function that ignores network and address and
// instead connects to the fixed address.
//
// Apps may still use custom resolvers (including secure resolvers) so this does not
// work very well - setting resolv.conf or interception still better.
func DNSDialer(port int) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{}
		//return d.DialContext(ctx, "udp", "1.1.1.1:53")
		return d.DialContext(ctx, "udp", fmt.Sprintf("127.0.0.1:%d", port))
	}
}

// Special capture for DNS with TUN or TPROXY. Will use the DNS VPN or direct calls.
func (gw *DmDns) HandleUdp(dstAddr net.IP, dstPort uint16,
	localAddr net.IP, localPort uint16,
	data []byte) {
	req := new(dns.Msg)
	req.Unpack(data)

	res := gw.Do(req)

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
