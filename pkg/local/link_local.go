package local

import (
	"bytes"
	"encoding/json"
	"errors"
	"expvar"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/costinm/ugate"
)


var (
	// Address used for link local discovery.
	// Multicast on port 5227
	MulticastDiscoveryIP6 = net.ParseIP("FF02::5227")
	MulticastDiscoveryIP4 = net.ParseIP("224.0.0.250")
)

var (
	// Received MC announcements from other nodes (include invalid)
	regDN = expvar.NewInt("RegD")

	// Error MC announcements from local nodes
	regDNE = expvar.NewInt("RegDE")

	// Client requests to check a peer.
	regDNC = expvar.NewInt("RegDC")

	// Errors Client requests to check a peer.
	regDNCE = expvar.NewInt("RegDCE")
)


func NewLocal(gw *ugate.UGate, auth *ugate.Auth) *LLDiscovery {
	return &LLDiscovery{
		mcPort:   5227,
		udpPort:  gw.Config.BasePort + 8,
		gw:       gw,
		auth:     auth,
	}
}

// Create a UDP listener for local UDP messages.
func ListenUDP(gw *LLDiscovery) {
	m2, err := net.ListenUDP("udp", &net.UDPAddr{Port: gw.udpPort})
	if err != nil {
		log.Println("Error listening on UDP ", gw.udpPort, err)
		m2, err = net.ListenUDP("udp", &net.UDPAddr{Port: 0})
		if err != nil {
			log.Println("Error listening on UDP ", gw.udpPort, err)
			return
		}
	}

	gw.UDPMsgConn = m2

	go unicastReaderThread(gw, m2, nil)
	go gw.PeriodicThread()
}



// Called after connection to the VPN has been created.
//
// Currently used only for Mesh AP chains.
func (gw *LLDiscovery) OnLocalNetworkFunc(node *ugate.DMNode, addr *net.UDPAddr, fromMySTA bool) {
	//now := time.Now()
	add := &net.UDPAddr{IP: addr.IP, Zone: addr.Zone, Port: 5222}

	if fromMySTA && node.TunClient == nil {
		log.Println("TODO: connect to ", add)
		//sshVpn, err := gw.gw.SSHGate.DialMUX(add.String(), node.PublicKey, nil)
		//if err != nil {
		//	log.Println("SSH STA ERR ", add, node.VIP, err)
		//	return
		//}
		//go func() {
		//	node.TunClient = sshVpn
		//
		//	// Blocking - will be closed when the ssh connection is closed.
		//	sshVpn.Wait()
		//
		//	node.TunClient = nil
		//	log.Println("SSH STA CLOSE ", add, node.VIP)
		//
		//}()
	}
}

// Format an address + zone + port for use in HTTP request
//
func (gw *LLDiscovery) FixIp6ForHTTP(addr *net.UDPAddr) string {
	if addr.IP.To4() != nil {
		return net.JoinHostPort(addr.IP.String(), strconv.Itoa(addr.Port))
	}
	if addr.Zone != "" {
		// Special code for the case the p2p- interface has changed.
		z := addr.Zone
		if strings.Contains(addr.Zone, "p2p-") && gw.ActiveP2P != "" {
			z = gw.ActiveP2P
		}

		return net.JoinHostPort(addr.IP.String()+"%25"+z, strconv.Itoa(addr.Port))
	}

	return addr.String()
}

// If we have a p2p interface, attempt to connect to the AP. Will work if this device isn't
// an AP itself, otherwise we need to wait for the connection from the AP to client
// or IP6 announce.
//
// Should be called after network changes and announce
func (gw *LLDiscovery) ensureConnectedUp(laddr *net.UDPAddr, node *ugate.DMNode) error {
	//if gw.gw.SSHClientUp != nil {
	//	return nil
	//}

	//var err error
	//for {
	//	hasUp := false
	//	for _, a := range gw.ActiveInterfaces {
			// WLAN connected to 49.1 - probably AP
			//if !strings.Contains(a.Name, "p2p") && a.AndroidAPClient {
			//	hasUp = true
			//	var conMux ugate.MuxedConn
			//	addr := ""
			//	if laddr != nil {
			//		addr = laddr.String()
			//		conMux, err = gw.gw.SSHGate.DialMUX(addr, node.PublicKey, nil)
			//	}
			//	if conMux == nil {
			//		addr = "192.168.49.1:5222"
			//		conMux, err = gw.gw.SSHGate.DialMUX(addr, nil, nil)
			//	}
			//	if err != nil {
			//		log.Println("MCDirect: announce P2P ToAP 49.1 fail ", err)
			//		time.Sleep(120 * time.Second)
			//		continue
			//	} else {
			//		log.Println("MCDirect: announce P2P ToAP 49.1 ok ")
			//		msgs.Send("/LM/P2P", "Addr",
			//			addr, "up", conMux.RemoteVIP().String())
			//
			//		gw.gw.SSHClientUp = conMux
			//
			//		// Blocking - will be closed when the ssh connection is closed.
			//		conMux.Wait()
			//		gw.gw.SSHClientUp = nil
			//		// TODO: shorter timeout with exponential backoff
			//		log.Println("SSH VPN CLOSED")
			//		time.Sleep(5 * time.Second)
			//	}
			//}
	//	}
	//	if !hasUp {
	//		return nil
	//	}
	//}
	return nil
}

// Local (non-internet) addresses.
// RFC1918, RFC4193, LL
func IsRFC1918(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.To4() == nil {
		// IPv6 equivalent - RFC4193, ULA - but this is used as DMesh
		if ip[0] == 0xfd {
			return true
		}
		if ip[0] == 0xfe && ip[1] == 0x80 {
			return true
		}
		return false
	}
	if ip[0] == 10 {
		return true
	}
	if ip[0] == 192 && ip[1] == 168 {
		return true
	}
	if ip[0] == 172 && ip[1]&0xF0 == 0x10 {
		return true
	}
	// Technically not 1918, but 6333
	if ip[0] == 192 && ip[1] == 0 && ip[2] == 0 {
		return true
	}

	return false
}

func (gw *LLDiscovery) isMeshClient() bool {
	for _, n := range gw.ActiveInterfaces {
		if n.AndroidAPClient {
			return true
		}
	}
	return false
}

// Return public IPs on the active interfaces.
func ips(interfaces map[string]*ActiveInterface) []*net.UDPAddr {
	res := []*net.UDPAddr{}
	for _, a := range interfaces {
		for _, i6 := range a.IPPub {
			if i6[0] == 0xfd {
				continue
			}
			u := &net.UDPAddr{IP: i6, Port: 5222}
			res = append(res, u)
		}
		if a.IP4 != nil && !IsRFC1918(a.IP4) {
			res = append(res, &net.UDPAddr{IP: a.IP4, Port: 5222})
		}
	}
	return res
}

// Remaining of the file deals with the MC announce and receiving, and tracking interfaces.
// registerLinkLocal is called when other nodes are found on the same net, to sync up.

// Message sent as a broadcast in the link local network.
// Current format:
// JSON
// 64 byte public key
// 64 byte signature
func mcMessage(gw *LLDiscovery, i *ActiveInterface, isAck bool) []byte {
	buf := new(bytes.Buffer)

	// VPN VIP
	// my client ssid
	//
	ann := &ugate.NodeAnnounce{
		UA:   gw.gw.Auth.Name,
		IPs:  ips(gw.ActiveInterfaces),
		//SSID: gw.auth.Config.Conf(gw.auth.Config, "ssid", ""),
		Ack:  isAck,
	}

	if i.AndroidAP || strings.Contains(i.Name, "p2p") {
		ann.AP = true
	}

	json.NewEncoder(buf).Encode(ann)
	return signedMessage(buf, gw.auth)
}

// Sign the message in the buffer.
func signedMessage(buf *bytes.Buffer, auth *ugate.Auth) []byte {

	buf.Write(auth.Pub[1:])
	buf.Write(auth.Pub[1:]) // to add another 64 bytes

	res := buf.Bytes()
	resLen := len(res)

	auth.Sign(res[0:resLen-64], res[resLen-64:])
	return res
}

// Debug: periodic MC and check
func (gw *LLDiscovery) periodicMC() {
	ticker := time.NewTicker(300 * time.Second)
	quit := make(chan struct{})

	time.AfterFunc(5*time.Second, func() {
		gw.AnnounceMulticast()
	})

	for {
		select {
		case <-ticker.C:
			gw.AnnounceMulticast()
		case <-quit:
			return
		}
	}
}

var (
	refreshMutex sync.RWMutex
)

//
//// Listen using UDP4, bound to a specific interface
//func listen4Priv(base int, ip net.IP, iface *net.Interface) (int, net.PacketConn) {
//	var err error
//
//	s, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
//	if err != nil {
//		log.Println("/ll/listen4.1", err)
//		return 0, nil
//	}
//	if err := syscall.SetsockoptInt(s, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
//		log.Println("/ll/listen4.2", err)
//		return 0, nil
//	}
//	//syscall.SetsockoptInt(s, syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
//	if err := syscall.SetsockoptString(s, syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface.Name); err != nil {
//		log.Println("/ll/listen4.3", err)
//		return 0, nil
//	}
//
//	lsa := syscall.SockaddrInet4{Port: base}
//	copy(lsa.Addr[:], []byte{0, 0, 0, 0})
//
//	if err := syscall.Bind(s, &lsa); err != nil {
//		syscall.Close(s)
//		log.Println("/ll/listen4.4", err)
//		return 0, nil
//	}
//
//	f := os.NewFile(uintptr(s), "")
//	c, err := net.FilePacketConn(f)
//	f.Close()
//	if err != nil {
//		log.Println("/ll/listen4", err)
//		return 0, nil
//	}
//	//p := ipv4.NewPacketConn(c)
//
//	return base, c
//}

// helper to listen on a base port on a specific interface only.
func (gw *LLDiscovery) listen6(base int, ip net.IP, iface *ActiveInterface) (int, *net.UDPConn) {
	var err error
	var m *net.UDPConn
	for i := 0; i < 10; i++ {
		udpAddr := &net.UDPAddr{
			IP:   ip,
			Port: base,
			Zone: iface.Name,
		}

		m, err = net.ListenUDP("udp6", udpAddr)

		if err == nil {
			go unicastReaderThread(gw, m, iface)
			return m.LocalAddr().(*net.UDPAddr).Port, m
		} else {
			log.Println("MCDirect: Port in use or error, ", iface, base, err)
			if m != nil {
				m.Close()
			}
		}
		base++
	}

	return 0, nil
}

// Listen on link-local interface using UDP4
func (gw *LLDiscovery) listen4(base int, ip net.IP, iface *ActiveInterface) (int, net.PacketConn) {
	var err error
	var m *net.UDPConn
	for i := 0; i < 10; i++ {
		udpAddr := &net.UDPAddr{
			IP:   ip,
			Port: base,
			Zone: iface.Name,
		}

		m, err = net.ListenUDP("udp4", udpAddr)

		if err == nil {
			go unicastReaderThread(gw, m, iface)
			return m.LocalAddr().(*net.UDPAddr).Port, m
		} else {
			log.Println("MCDirect: Port in use or error, ", iface, base, err)
			if m != nil {
				m.Close()
			}
		}
		base++
	}

	return 0, nil
}

// Sends a packet to dmesh routers, using local multicast.
// Called when refreshNetworks is called, 15 min or on change
func (gw *LLDiscovery) AnnounceMulticast() {
	if gw.UDPMsgConn == nil {
		log.Println("Invalid UDPMSGConn")
		return
	}
	cnt := 0
	ok := []string{}
	fail := []string{}

	for _, a := range gw.ActiveInterfaces {
		a := a

		if a.unicastUdpServer == nil && a.unicastUdpServer4 == nil {
			log.Println("MCDirect: AnnounceMulticast: no source socket", a)
			fail = append(fail, a.Name)
			continue
		}

		addr := &net.UDPAddr{
			IP:   MulticastDiscoveryIP6,
			Port: gw.mcPort,
			Zone: a.iface.Name,
		}
		addr2 := &net.UDPAddr{
			IP:   MulticastDiscoveryIP4,
			Port: gw.mcPort,
			//Zone: a.iface.Name,
		}
		jsonAnn := mcMessage(gw, a, false)

		var err error
		for i := 0; i < 2; i++ {

			gw.UDPMsgConn.WriteTo(jsonAnn, addr)

			//if a.unicastUdpServer != nil {
			//	_, err = a.unicastUdpServer.WriteTo(jsonAnn, addr)
			//	if err == nil {
			//		cnt++
			//		time.Sleep(50 * time.Millisecond)
			//	}
			//	if err != nil {
			//		log.Println("MCDirect: Failed to send6 direct register", err, addr)
			//	}
			//}
			if a.unicastUdpServer4 != nil {
				_, err = a.unicastUdpServer4.WriteTo(jsonAnn, addr2)
				if err == nil {
					cnt++
					time.Sleep(50 * time.Millisecond)
				}
				if err != nil {
					log.Println("MCDirect: Failed to send4 direct register", err, addr2)
				}
			}
		}
		if err != nil {
			log.Println("MCDirect: Failed to send direct register", err, addr2)
			fail = append(fail, a.Name)
		} else {
			ok = append(ok, a.Name)
		}
	}
	log.Println("/dis/ann", cnt, ok, fail)

	// if any interface is AP client ( 49.x but not 1) - attempt to connect AP
	go gw.ensureConnectedUp(nil, nil)
}

var (
	selfRegister = errors.New("Self register")
	dup          = errors.New("Dup")
)

// Multicast registration server. One for each interface
func unicastReaderThread(gw *LLDiscovery, c net.PacketConn, iface *ActiveInterface) {
	iname := ""
	if iface != nil {
		iname = iface.iface.Name
	}
	defer func() {
		c.Close()
		log.Println("MCDirect: unicastReaderThread master closed ", iname)
	}()

	m := make([]byte, 1600)
	for {
		m = m[0:1600]
		n, addr1, err := c.ReadFrom(m) //c.ReadFromUDP(m)
		if err != nil {
			log.Println("MCDirect: unicast dmesh receive error: ", iname, err)
			return
		}
		rcv := m[0:n]
		if len(rcv) < 129 {
			continue
		}

		addr, _ := addr1.(*net.UDPAddr)

		directNode, ann, err := gw.processMCAnnounce(rcv, addr, iface)
		if err != nil {
			if err != selfRegister && err != dup {
				log.Println("MCDirect: Invalid multicast  ", err, addr, n, string(rcv[0:len(rcv)-128]))
			}
			continue
		}

		if addr.IP.To4() != nil {
			if time.Now().Sub(directNode.LastSeen4) < 10*time.Second {
				continue
			}
			directNode.LastSeen4 = directNode.LastSeen
			directNode.Last4 = addr
		} else {
			if time.Now().Sub(directNode.LastSeen6) < 10*time.Second {
				continue
			}
			directNode.LastSeen6 = directNode.LastSeen
			directNode.Last6 = addr

			if ann.AP {
				log.Println("Attempt to connect to AP using IP6 LL")
				go gw.ensureConnectedUp(addr, directNode)
			}
		}

		log.Println("LL: ACK Received:", directNode.VIP, c.LocalAddr(), addr, ann)

	}
}

// Multicast registration server. One for each interface
func (gw *LLDiscovery) multicastReaderThread(c net.PacketConn, iface *ActiveInterface) {
	defer func() {
		c.Close()
		log.Println("MCDirect: multicatReaderThread master closed ", iface.IP6LL, iface.iface.Name)
	}()
	if gw.UDPMsgConn == nil {
		log.Print("INVALID UDP LL")
		return
	}
	m := make([]byte, 1600)
	for {
		m = m[0:1600]
		// It seems this gets blocked here, and more routines accumulate.
		n, addr1, err := c.ReadFrom(m) //c.ReadFromUDP(m)
		if err != nil {
			log.Println("MCDirect: dmesh receive error: ", iface, err)
			return
		}
		rcv := m[0:n]

		addr, _ := addr1.(*net.UDPAddr)
		if addr.Zone != "" && addr.Zone != iface.iface.Name {
			continue
		}
		port0 := addr.Port

		regDN.Add(1)

		directNode, ann, err := gw.processMCAnnounce(rcv, addr, iface)
		if err != nil {
			if err != selfRegister && err != dup {
				log.Println("MCDirect: Invalid multicast  ", err, addr, n, string(rcv[0:len(rcv)-128]))
			}
			continue
		}

		// Received from a STA, on my AP interface. Due to LL bug, need workaround.
		// (this is also in the low-level QUIC stack)
		// Use the port from the announce - may be different from the sent port
		// (for example if a different server like envoy is handling QUIC)
		//addr = &net.UDPAddr{IP: addr.IP, Port: dn.NodeAnnounce.Port, Zone: addr.Zone}

		//addr = &net.UDPAddr{IP: addr.IP, Port: 5222, Zone: iface.iface.Name} // instead if add.Zone

		//if strings.Contains(addr.Zone, "p2p") {
		//	addr.IP = androidClientUnicast2MulticastAddress(addr.IP)
		//	addr.Port = dn.NodeAnnounce.APort
		//}

		// For a 'direct' child, if this node is not an AP/GW, all we really need is a map
		// from uint24 session ID to IP6 GW address.
		// The child only need the parent IP6 address (selected from multiple responses)

		// For a GW node, the session ID is enough as well, unless a whitelist and
		// auth is used for access.

		// Current android doesn't support arbitrary ports
		// For internet, to keep NAT alive the registration should be sent from 5228.

		//if VerboseAnnounce {
		//	log.Println("MCDirect: Received MC Announce: ", addr, ann, directNode, iface)
		//}

		if addr.IP.To4() != nil {
			if time.Now().Sub(directNode.LastSeen4) < 10*time.Second {
				continue
			}
			directNode.LastSeen4 = directNode.LastSeen
			directNode.Last4 = addr
		} else {
			if time.Now().Sub(directNode.LastSeen6) < 10*time.Second {
				continue
			}
			directNode.LastSeen6 = directNode.LastSeen
			directNode.Last6 = addr
		}

		// Observed behavior: android AP doesn't send multicasts to connected clients.
		// It does receive them, and can send back ACK.
		isAp := directNode.NodeAnnounce.AP && gw.isMeshClient()
		if isAp {
			go gw.ensureConnectedUp(addr, directNode)
		}

		gw.OnLocalNetworkFunc(directNode, addr, strings.Contains(addr.Zone, "p2p"))

		log.Println("LL: MC Received:", directNode.VIP, addr, directNode.NodeAnnounce.UA, isAp)

		if !ann.Ack {
			// Send an 'ack' back to the device, including our own ID/sig and info
			addr.Port = port0
			jsonAnn := mcMessage(gw, iface, true)

			gw.UDPMsgConn.WriteTo(jsonAnn, addr)
			// This works for IP6 - not for IP4
			//
			//
			//if addr.IP.To4() == nil && iface.unicastUdpServer != nil {
			//	_, err = iface.unicastUdpServer.WriteTo(jsonAnn, addr)
			//}
			//if addr.IP.To4() != nil && iface.unicastUdpServer4 != nil {
			//	_, err = iface.unicastUdpServer4.WriteTo(jsonAnn, addr)
			//}
			addr.Port = gw.udpPort
			//if addr.IP.To4() == nil && iface.unicastUdpServer != nil {
			//	_, err = iface.unicastUdpServer.WriteTo(jsonAnn, addr)
			//}
			//if addr.IP.To4() != nil && iface.unicastUdpServer4 != nil {
			//	_, err = iface.unicastUdpServer4.WriteTo(jsonAnn, addr)
			//}

			gw.UDPMsgConn.WriteTo(jsonAnn, addr)
		}
	}
}

var errZone = errors.New("same zone")
var errStart = errors.New("invalid start of message")
var errMinSize = errors.New("too short")

func (gw *LLDiscovery) HttpGetLLIf(w http.ResponseWriter, r *http.Request) {
	gw.RefreshNetworks()

	gw.activeMutex.RLock()
	defer gw.activeMutex.RUnlock()
	je := json.NewEncoder(w)
	je.SetIndent(" ", " ")
	je.Encode(gw.ActiveInterfaces)
}

// AddHandler a UDP multicast announce. Used when this discovery server acts as a registry (master).
// The content of the multicasts is signed for historical reasons, due to the older version, where
// it could be forwarded and re-sent.
//
// Currently the info is only for debugging - all registration happens in the /register handshake,
// using mtls.
func (gw *LLDiscovery) processMCAnnounce(data []byte, addr *net.UDPAddr, iface *ActiveInterface) (*ugate.DMNode, *ugate.NodeAnnounce, error) {

	dl := len(data)

	if dl < 64+64+2 {
		return nil, nil, errMinSize
	}

	if data[0] != '{' {
		return nil, nil, errStart
	}

	jsonData := data[0 : dl-128]
	pub := data[dl-128 : dl-64]
	sig := data[dl-64 : dl]

	// Check signature. Verified Public key is the identity
	err := ugate.Verify(data[0:dl-64], pub, sig)
	if err != nil {
		log.Println("MCDirect: Signature ", err)
		return nil, nil, err
	}

	dmFrom := ugate.Pub2ID(pub)
	if gw.auth.VIP64 == dmFrom {
		return nil, nil, selfRegister
	}

	// Parse the message
	ann := &ugate.NodeAnnounce{}
	err = json.Unmarshal(jsonData, ann)
	if err != nil {
		log.Println("MCDirect: Failed to parse ann", err, string(data[0:dl-128]))
	}

	now := time.Now()

	node := gw.gw.GetOrAddNode(ugate.IDFromPublicKey(pub))

	since := int(now.Sub(node.LastSeen) / time.Second)
	if since > 2 {
		node.Seen = append(node.Seen, since)
	}
	node.LastSeen = now
	if len(node.Seen) > 100 {
		node.Seen = node.Seen[1:]
	}

	node.NodeAnnounce = ann
	// ???

	node.Announces++
	if strings.Contains(addr.Zone, "p2p") {
		node.AnnouncesOnP2P++
	}
	if ann.AP {
		node.AnnouncesFromP2P++
	}

	// IP4 addresses don't include zone for some reason...
	if addr.Zone != "" && iface != nil && iface.Name != addr.Zone {
		log.Println("MCDirect: Missmatch iface and GW ", addr, iface)
	}

	return node, ann, nil
}

var (
	VerboseAnnounce = true
)

// For an interface, return the LinkLocalUnicast IPv6 addres, or nil
func ip6Local(ifi net.Interface) (net.IP, net.IP, []net.IP) {
	addrs, err := ifi.Addrs()
	if err != nil {
		log.Println("MCDirect: Error getting local address", ifi, err)
		return nil, nil, nil
	}
	if isLoopback(addrs) {
		return nil, nil, nil
	}
	if strings.Contains(ifi.Name, "dmesh") {
		return nil, nil, nil
	}

	ip4 := net.IP(nil)
	ip6LL := net.IP(nil)
	res := []net.IP{}

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		// net.Addr - can be net.IPAddr(IP6 + zone string) or net.IPNet (IP + mask)
		if ipnet.IP.To4() == nil {
			if ipnet.IP.IsLinkLocalUnicast() { // fe80:: or 169.254.x.x
				ip6LL = ipnet.IP
			} else {
				res = append(res, ipnet.IP)
			}
		} else {
			if ipnet.IP.IsLinkLocalUnicast() { // fe80:: or 169.254.x.x
				continue
			}
			ip4 = ipnet.IP.To4()
		}

	}
	return ip6LL, ip4, res
}

func isLoopback(addrs []net.Addr) bool {
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			return true
		}
	}
	return false
}

// Extract IPv4 addresses from a list
func ip4(addrs []net.Addr) net.IP {
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && ipnet.IP.To4() != nil {
			return ipnet.IP.To4()
		}
	}
	return nil
}

//func (lm *LinkLocalRegistry) getIf(zone string) *ActiveInterface {
//	lm.activeMutex.Lock()
//	defer lm.activeMutex.Unlock()
//	a, found := lm.ActiveInterfaces[zone]
//	if !found {
//		log.Println("Node no longer ActiveInterface ", zone)
//		return nil
//	}
//	return a
//}
