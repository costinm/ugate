package local_discovery

import (
	"bytes"
	"crypto"
	"encoding/json"
	"errors"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/costinm/meshauth/pkg/certs"
	"github.com/costinm/meshauth/pkg/tokens"
)

var (
	// Address used for link local discovery.
	// Multicast on port 5227
	MulticastDiscoveryIP6 = net.ParseIP("FF02::5227")
	MulticastDiscoveryIP4 = net.ParseIP("224.0.0.250")
)

type LLDiscovery struct {
	udpPort      int

	UDPMsgConn   *net.UDPConn `json:"-"`

	activeMutex  sync.RWMutex
	refreshMutex sync.RWMutex

	ActiveInterfaces map[string]*ActiveInterface `json:"-"`

	Name             string `json:"name,omitempty"`

	ActiveP2P        string `json:"-"`

	baseListenPort   int
	mcPort           int

	VIP64            uint64 `json:"-"`

	Nodes map[string]*Node `json:"-"`

	pub   []byte
	priv  crypto.PrivateKey
}

// Starts create a UDP listener for local UDP messages, used for
// direct reply and ACK.
//
// Also starts a periodic task to find the interfaces and send multicast
// messages on each network. Each interface will get a listener for that
// interface.
func (disc *LLDiscovery) Start() error {
	m2, err := net.ListenUDP("udp", &net.UDPAddr{Port: disc.udpPort})
	if err != nil {
		log.Println("Error listening on UDP ", disc.udpPort, err)
		m2, err = net.ListenUDP("udp", &net.UDPAddr{Port: 0})
		if err != nil {
			log.Println("Error listening on UDP ", disc.udpPort, err)
			return err
		}
	}

	disc.UDPMsgConn = m2

	go unicastReaderThread(disc, m2, nil)

	go disc.PeriodicThread()
	return nil
}

// Periodic refresh and registration.
func (disc *LLDiscovery) PeriodicThread() error {
	// TODO: dynamically adjust the timer
	// TODO: CON and AP events should be sufficient - check if Refresh is picking anything
	// new.
	ticker := time.NewTicker(120 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				disc.RefreshNetworks()
				// TODO: keepalive or register on VPN, if ActiveInterface (may use different timer)
			case <-quit:
				return
			}
		}
	}()

	disc.RefreshNetworks()
	return nil
}

var brokenAndroidNetworks = false

// RefreshNetworks will update the list of ActiveInterface networks, and ensure each has a listener.
// Local communication uses the link-local address. If the interface
// is connected to an Android AP, it uses a link-local multicast address instead.
//
// - Called from android using "r" message, on connectivity changes
// - Also called from android at startup and property changes ( "P" - properties ).
// - 15-min thread on link local
func (disc *LLDiscovery) RefreshNetworks() {
	if brokenAndroidNetworks {
		go disc.AnnounceMulticast()
		return
	}

	disc.refreshMutex.Lock()
	defer disc.refreshMutex.Unlock()

	t0 := time.Now()
	newAct, err := ActiveNetworks(disc)
	if err != nil {
		// In android ActiveNetworks doesn't work, permission denied.
		log.Println("Error getting active networks ", err)
		brokenAndroidNetworks = true
		// Fallback
		go disc.AnnounceMulticast()

		log.Println("MCDirect: RefreshNetworks", time.Since(t0))
		return
	}

	unchanged := map[string]*ActiveInterface{}

	disc.activeMutex.Lock()
	defer disc.activeMutex.Unlock()

	//changed := false

	// True if any of the interfaces is an Android AP
	hasAp := false

	// For disconnected networks - close the sockets and stop listening
	for nname, existing := range disc.ActiveInterfaces {
		a := findActive(existing, newAct)

		if a != nil {
			unchanged[nname] = existing
			// Pass the udp socket
			a.multicastRegisterConn = existing.multicastRegisterConn
			a.multicastRegisterConn2 = existing.multicastRegisterConn2
			a.unicastUdpServer = existing.unicastUdpServer
			a.unicastUdpServer4 = existing.unicastUdpServer4
			//a.tcpListener = existing.tcpListener
			//a.multicastUdpServer = existing.multicastUdpServer
			a.Port = existing.Port
			continue
		}

		//changed = true
		// No longer ActiveInterface. The socket is probably closed already
		log.Println("MCDirect: Interface no longer active ", existing)
		if existing.unicastUdpServer != nil {
			existing.unicastUdpServer.Close()
		}
		if existing.unicastUdpServer4 != nil {
			existing.unicastUdpServer4.Close()
		}
		if existing.multicastRegisterConn != nil {
			existing.multicastRegisterConn.Close()
		}
		if existing.multicastRegisterConn2 != nil {
			existing.multicastRegisterConn2.Close()
		}
	}

	names := []string{}
	for _, a := range newAct {
		//if /*a.iface.Name == "p2p0" && */ a.iface.Name != lm.Register.Gateway.ActiveP2P {
		//	// Otherwise 'address in use' attempting to listen on android AP
		//	// New devices use p2p0. Older have p2p0 and another one
		//	continue
		//}
		if a.Port == 0 {
			a.Port = disc.baseListenPort
		}
		if a.AndroidAP {
			hasAp = true
		}
		names = append(names, a.Name)

		if a.unicastUdpServer == nil && a.IP6LL != nil {
			//changed = true
			port, ucListener := disc.listen6(disc.baseListenPort, a.IP6LL, a)
			if ucListener == nil {
				//slog.Warn("MCDirect: Failed to start unicast server ", "err", err, "addr", gw.baseListenPort, "i", a.IP6LL)
			} else {
				a.Port = port
				a.unicastUdpServer = ucListener
			}
		}

		if a.unicastUdpServer4 == nil && a.IP4 != nil {
			//changed = true
			port4, ucListener4 := disc.listen4(disc.baseListenPort, a.IP4, a)
			if ucListener4 == nil {
				log.Println("MCDirect: Failed to start unicast4 server ", err)
			} else {
				a.Port4 = port4
				a.unicastUdpServer4 = ucListener4
			}

		}

		//log.Printf("MCDirect: unicast %s [%s]:%d %s:%d", a.iface.Name, a.IP6LL, a.Port, a.IP4, a.Port4)

		// Initiate the UDP socket used to send MC registration requests and receive local QUIC connections.
		//if a.multicastUdpServer == nil {
		//	changed = true
		//	// startDirectServer creates the direct UDP unicast DmDns for the node
		//	ip := androidClientUnicast2MulticastAddress(a.IP6LL)
		//	port, mcListener, _ := listen(a.Port+1, ip, a.iface, true)
		//	if mcListener == nil {
		//		log.Println("MCDirect: Failed to start AndroidAP multicast server ", err)
		//		continue
		//	} else {
		//		log.Println("MCDirect: multicast server", ip, port, a.iface.Name, a.IP4)
		//		a.multicastUdpServer = mcListener
		//		if transport.UseQuic {
		//			lm.H2.InitQuicServerConn(port, mcListener, lm.H2.MTLSMux)
		//		}
		//	}
		//}

		if true || hasAp {

			if a.multicastRegisterConn != nil {
				continue // was ActiveInterface before
			}
			if a.iface.Flags&net.FlagMulticast == 0 {
				continue
			}
			mc6APUDPAddr := &net.UDPAddr{
				IP:   MulticastDiscoveryIP6,
				Port: disc.mcPort,
				Zone: a.iface.Name,
			}
			mc4APUDPAddr := &net.UDPAddr{
				IP:   MulticastDiscoveryIP4,
				Port: disc.mcPort,
				Zone: a.iface.Name,
			}
			if a.IP6LL != nil {
				m, err := net.ListenMulticastUDP("udp6", a.iface, mc6APUDPAddr)
				if err != nil {
					log.Println("MCDirect: Failed to start multicast6 ", a, err)
				} else {
					//log.Println("MCDirect: MASTER: ", a.IP6LL, a.iface.Name, mc6APUDPAddr, a.IP4, a.AndroidAP)
					a.multicastRegisterConn = m
					a := a
					go disc.multicastReaderThread(m, a)
				}
			}

			if a.IP4 != nil && a.multicastRegisterConn2 == nil {
				//mc1 := ipv4.NewPacketConn(a.multicastRegisterConn2)
				//if err := mc1.JoinGroup(a.iface, &net.UDPAddr{IP: mc4APUDPAddr.IP}); err != nil {
				//	log.Println("MCDirect: Failed to start multicast4 ", a, err)
				//	//log.Fatal(err)
				//}
				//
				//if err := mc1.SetControlMessage(ipv4.FlagTTL|ipv4.FlagSrc|ipv4.FlagDst|ipv4.FlagInterface, true); err != nil {
				//	log.Println("MCDirect: Failed to start multicast4 ", a, err)
				//	//log.Fatal(err)
				//}

				m2, err := net.ListenMulticastUDP("udp4", a.iface, mc4APUDPAddr)
				//m2 := a.multicastRegisterConn2
				if err != nil {
					log.Println("MCDirect: Failed to start multicast4 ", a, err)
				} else {
					//log.Println("MCDirect: MASTER4: ", a.IP6LL, a.iface.Name, mc4APUDPAddr, a.IP4, a.AndroidAP)
					a.multicastRegisterConn2 = m2
					go disc.multicastReaderThread(m2, a)
				}
			}
		} else {
			// Not master - shut down the multicast socket
			if a.multicastRegisterConn != nil {
				log.Println("MCDirect: stop (master off) ", a.IP6LL, a.iface.Name)
				a.multicastRegisterConn.Close()
				a.multicastRegisterConn = nil
				a.multicastRegisterConn2.Close()
				a.multicastRegisterConn2 = nil
			}
		}
	}
}

// // Format an address + zone + port for use in HTTP request
func (disc *LLDiscovery) FixIp6ForHTTP(addr *net.UDPAddr) string {
	if addr.IP.To4() != nil {
		return net.JoinHostPort(addr.IP.String(), strconv.Itoa(addr.Port))
	}
	if addr.Zone != "" {
		// Special code for the case the p2p- interface has changed.
		z := addr.Zone
		if strings.Contains(addr.Zone, "p2p-") && disc.ActiveP2P != "" {
			z = disc.ActiveP2P
		}

		return net.JoinHostPort(addr.IP.String()+"%25"+z, strconv.Itoa(addr.Port))
	}

	return addr.String()
}

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
			//MetricLLReceiveErr.Add(1)
			//if err != selfRegister && err != dup {
			//	log.Println("MCDirect: Invalid multicast  ", err, addr, n)
			//}
			continue
		}
		//MetricLLReceived.Add(1)

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
				//go gw.ensureConnectedUp(addr, directNode)
			}
		}

		//MetricLLTotal.Set(int64(len(gw.Nodes)))

		log.Println("LL: ACK Received:", directNode.ID, c.LocalAddr(), addr, ann)
		//MetricLLReceivedAck.Add(1)
	}
}

// helper to listen on a base port on a specific interface only.
func (disc *LLDiscovery) listen6(base int, ip net.IP, iface *ActiveInterface) (int, *net.UDPConn) {
	var err error
	var m *net.UDPConn
	for i := 0; i < 10; i++ {
		udpAddr := &net.UDPAddr{
			IP:   ip,
			Port: base + i,
			Zone: iface.Name,
		}

		m, err = net.ListenUDP("udp6", udpAddr)

		if err == nil {
			go unicastReaderThread(disc, m, iface)
			return m.LocalAddr().(*net.UDPAddr).Port, m
		} else {
			//log.Println("MCDirect: Port in use or error, ", iface, base, err)
			if m != nil {
				m.Close()
			}
		}
		base++
	}

	return 0, nil
}

// Listen on link-local interface using UDP4
func (disc *LLDiscovery) listen4(base int, ip net.IP, iface *ActiveInterface) (int, net.PacketConn) {
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
			go unicastReaderThread(disc, m, iface)
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

// ActiveInterface tracks one 'up' interface. Used for IPv6 multicast,
// which requires 'zone', and to find the local addresses.
// On recent Android - it is blocked by privacy and not used.
type ActiveInterface struct {
	// Interface name. Name containing 'p2p' results in specific behavior.
	Name string

	// IP6 link local address. May be nil if IPPub is set.
	// One or the other must be set.
	IP6LL net.IP

	// IP4 address - may be a routable address, nil or private address.
	// If public address - may be included in the register, but typically not
	// useful.
	IP4 net.IP

	// Public addresses. IP6 address may be used for direct connections (in some
	// cases)
	IPPub []net.IP

	// Port for the UDP unicast link-local listener.
	Port int
	// Port for the UDP unicast link-local listener.
	Port4 int

	// True if this interface is an Android AP
	AndroidAP bool

	// True if this interface is connected to an Android DM node.
	AndroidAPClient bool

	// interface
	iface *net.Interface

	// Set if the interface is listening for MC packets, as master.
	multicastRegisterConn net.PacketConn
	// Set if the interface is listening for MC packets, as master.
	multicastRegisterConn2 net.PacketConn

	// Packet conn bound to the unicast link-local address, or in case of android the special multicast
	// link local
	unicastUdpServer  net.PacketConn
	unicastUdpServer4 net.PacketConn
	//multicastUdpServer net.PacketConn

	//tcpListener *net.TCPListener

}

// Debug: periodic MC and check
func (disc *LLDiscovery) periodicMC() {
	ticker := time.NewTicker(300 * time.Second)
	quit := make(chan struct{})

	time.AfterFunc(5*time.Second, func() {
		disc.AnnounceMulticast()
	})

	for {
		select {
		case <-ticker.C:
			disc.AnnounceMulticast()
		case <-quit:
			return
		}
	}
}

// Information about a node.
// Sent periodically, signed by the origin - for example as a JWT, or UDP
// proto.
// TODO: map it to Pod, IPFS announce
// TODO: move Wifi discovery to separate package.
type NodeAnnounce struct {
	UA string `json:"UA,omitempty"`

	// Non-link local IPs from all interfaces. Includes public internet addresses
	// and Wifi IP4 address. Used to determine if a node is directly connected.
	IPs []*net.UDPAddr `json:"addrs,omitempty"`

	// Set if the node is an active Android AP.
	SSID string `json:"ssid,omitempty"`

	// True if the node is an active Android AP on the interface sending the message.
	// Will trigger special handling in link-local - if the receiving interface is also
	// an android client.
	AP bool `json:"AP,omitempty"`

	Ack bool `json:"ACK,omitempty"`

	// VIP of the direct parent, if this node is connected.
	// Used to determine the mesh topology.
	Vpn string `json:"Vpn,omitempty"`
}

type Node struct {
	ID       string
	LastSeen time.Time

	// In seconds since first seen, last 100
	Seen []int `json:"-"`
	// Information from the node - from an announce or message.
	// Not trusted, self-signed.
	NodeAnnounce *NodeAnnounce `json:"info,omitempty"`
	LastSeen4    time.Time
	Last4        *net.UDPAddr
	LastSeen6    time.Time
	Last6        *net.UDPAddr
}

var errZone = errors.New("same zone")
var errStart = errors.New("invalid start of message")
var errMinSize = errors.New("too short")

func (disc *LLDiscovery) processMCAnnounce(data []byte, addr *net.UDPAddr, iface *ActiveInterface) (*Node, *NodeAnnounce, error) {

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
	err := tokens.Verify(data[0:dl-64], pub, sig)
	if err != nil {
		return nil, nil, err
	}

	dmFrom := certs.Pub2ID(pub)
	if disc.VIP64 == dmFrom {
		return nil, nil, selfRegister
	}

	// Parse the message
	ann := &NodeAnnounce{}
	err = json.Unmarshal(jsonData, ann)
	if err != nil {
		log.Println("MCDirect: Failed to parse ann", err, string(data[0:dl-128]))
	}

	now := time.Now()
	id := certs.PublicKeyBase32SHA(pub)

	disc.activeMutex.RLock()
	node := disc.Nodes[id]
	disc.activeMutex.RUnlock()
	if node == nil {
		node = &Node{
			ID: id,
		}
		disc.activeMutex.Lock()
		disc.Nodes[id] = node
		disc.activeMutex.Unlock()
	}

	since := int(now.Sub(node.LastSeen) / time.Second)
	if since > 2 {
		node.Seen = append(node.Seen, since)
	}
	node.LastSeen = now
	if len(node.Seen) > 100 {
		node.Seen = node.Seen[1:]
	}

	node.NodeAnnounce = ann

	// IP4 addresses don't include zone
	if addr.Zone != "" && iface != nil && iface.Name != addr.Zone {
		log.Println("MCDirect: Missmatch iface and GW ", addr, iface)
	}

	return node, ann, nil
}

// Multicast registration server. One for each interface, the MC must
// be associated with the interface.
func (disc *LLDiscovery) multicastReaderThread(c net.PacketConn, iface *ActiveInterface) {
	defer func() {
		c.Close()
		log.Println("MCDirect: multicatReaderThread master closed ", iface.IP6LL, iface.iface.Name)
	}()

	if disc.UDPMsgConn == nil {
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

		//regDN.Add(1)

		directNode, ann, err := disc.processMCAnnounce(rcv, addr, iface)
		if err != nil {
			//regDNE.Add(1)
			//if err != selfRegister && err != dup {
			//	log.Println("MCDirect: ann err ", err, addr, n)
			//}
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
		// from uint24 session WorkloadID to IP6 GW address.
		// The child only need the parent IP6 address (selected from multiple responses)

		// For a GW node, the session WorkloadID is enough as well, unless a whitelist and
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
		//isAp := directNode.NodeAnnounce.AP && gw.isMeshClient()
		//if isAp {
		//	go gw.ensureConnectedUp(addr, directNode)
		//}

		//gw.OnLocalNetworkFunc(directNode, addr, strings.Contains(addr.Zone, "p2p"))

		log.Println("LL: MC Received:", directNode.ID, addr, directNode.NodeAnnounce.UA)

		if !ann.Ack {
			// Send an 'ack' back to the device, including our own WorkloadID/sig and info
			addr.Port = port0
			jsonAnn := mcMessage(disc, iface, true)

			disc.UDPMsgConn.WriteTo(jsonAnn, addr)
			// This works for IP6 - not for IP4
			//
			//
			//if addr.IP.To4() == nil && iface.unicastUdpServer != nil {
			//	_, err = iface.unicastUdpServer.WriteTo(jsonAnn, addr)
			//}
			//if addr.IP.To4() != nil && iface.unicastUdpServer4 != nil {
			//	_, err = iface.unicastUdpServer4.WriteTo(jsonAnn, addr)
			//}
			addr.Port = disc.udpPort
			//if addr.IP.To4() == nil && iface.unicastUdpServer != nil {
			//	_, err = iface.unicastUdpServer.WriteTo(jsonAnn, addr)
			//}
			//if addr.IP.To4() != nil && iface.unicastUdpServer4 != nil {
			//	_, err = iface.unicastUdpServer4.WriteTo(jsonAnn, addr)
			//}

			disc.UDPMsgConn.WriteTo(jsonAnn, addr)
		}
	}
}

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
	ann := &NodeAnnounce{
		UA:  gw.Name,
		IPs: ips(gw.ActiveInterfaces),
		//SSID: gw.auth.Config.Conf(gw.auth.Config, "ssid", ""),
		Ack: isAck,
	}

	if i.AndroidAP || strings.Contains(i.Name, "p2p") {
		ann.AP = true
	}

	json.NewEncoder(buf).Encode(ann)
	return signedMessage(buf, gw.pub, gw.priv)
}

// Sign the message in the buffer.
// pub is 64 bytes
func signedMessage(buf *bytes.Buffer, pub []byte, priv crypto.PrivateKey) []byte {

	buf.Write(pub)
	buf.Write(pub) // to add another 64 bytes

	res := buf.Bytes()
	resLen := len(res)

	sig := tokens.Sign(res[0:resLen-64], priv)
	// copy sig into res last 64 bytes
	copy(res[resLen-64:], sig)

	return res
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

func IPs() []net.IP {
	ips := []net.IP{}

	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err == nil {
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				ips = append(ips, ip)
				// process IP address
			}
		}
	}
	return ips
}

// ActiveNetworks lists the networks that have IP6 link local, and detects if they are
// android special or AP. (ap, p2pClient)
func ActiveNetworks(gw *LLDiscovery) (map[string]*ActiveInterface, error) {
	// TODO: at least on N9, rmnet_usb0 doesn't show any active IP
	anets := map[string]*ActiveInterface{}

	ifaces, err := net.Interfaces()
	if err != nil {
		// Android - denied on recent versions. "route ip+net: netlinkrib: permission denied"
		return nil, err
	}

	gw.ActiveP2P = ""

	for _, ifi := range ifaces {
		ifi := ifi
		if ifi.Flags & net.FlagUp == 0 {
			continue
		}
		if strings.Contains(ifi.Name, "dummy") {
			continue
		}
		ip, ip4, ippub := ip6Local(ifi)
		if ip == nil && ip4 == nil && len(ippub) == 0 {
			continue
		}
		if ip4 == nil && (ifi.Name == "p2p") {
			// Strange case of p2p not active
			continue
		}

		a := &ActiveInterface{
			Name:  ifi.Name,
			IP6LL: ip,
			IP4:   ip4,
			IPPub: ippub,
			iface: &ifi,
		}
		anets[ifi.Name] = a

		// Detection for Android AP. It could be &&, and check if it's android, however
		// detecting if it's client is trickier without knowing the SSID, so we can document
		// that an AP must not use '49'.
		if bytes.Equal(ip4, []byte{192, 168, 49, 1}) { // && strings.Contains(ifi.Name, "p2p") {
			a.AndroidAP = true
			gw.ActiveP2P = ifi.Name
			continue
		}

		//if strings.Contains(a.Name, "p2p") && ip4 != nil {
		//	//a.AndroidAP = true
		//	//lm.Register.Gateway.ActiveP2P = ifi.Name
		//	a.AndroidAPClient = true
		//	log.Println("P2P client with different IP ", a.Name, ip4)
		//	continue
		//}

		// Detect if it's android client
		// TODO: also check the wifi network name, if available.
		if ip4 != nil && ip4[0] == 192 && ip4[1] == 168 && ip4[2] == 49 {
			if ip4[3] != 1 {
				a.AndroidAPClient = true
			}
		}
	}

	//log.Println("XXX interfaces0: ", ifaces, anets)
	return anets, nil
}

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

func findActive(tofind *ActiveInterface, in map[string]*ActiveInterface) *ActiveInterface {
	if tofind == nil || in == nil {
		return nil
	}
	a, _ := in[tofind.iface.Name]
	return a
}

// Sends a packet to dmesh routers, using local multicast.
func (disc *LLDiscovery) AnnounceMulticast() {
	if disc.UDPMsgConn == nil {
		log.Println("Invalid UDPMSGConn")
		return
	}
	cnt := 0
	ok := []string{}
	fail := []string{}
	if len(disc.ActiveInterfaces) == 0 {
		addr := &net.UDPAddr{
			IP:   MulticastDiscoveryIP6,
			Port: disc.mcPort,
			//Zone: a.iface.Name,
		}
		jsonAnn := mcMessage(disc, nil, false)

		for i := 0; i < 2; i++ {
			_, err := disc.UDPMsgConn.WriteTo(jsonAnn, addr)
			if err != nil {
				log.Println("Failed to send MC1", addr)
			}
		}
		return
	}

	// Send IPv6 messages
	for _, a := range disc.ActiveInterfaces {
		a := a

		if a.unicastUdpServer == nil && a.unicastUdpServer4 == nil {
			log.Println("MCDirect: AnnounceMulticast: no source socket", a)
			fail = append(fail, a.Name)
			continue
		}

		addr := &net.UDPAddr{
			IP:   MulticastDiscoveryIP6,
			Port: disc.mcPort,
			Zone: a.iface.Name,
		}
		addr2 := &net.UDPAddr{
			IP:   MulticastDiscoveryIP4,
			Port: disc.mcPort,
			//Zone: a.iface.Name,
		}
		jsonAnn := mcMessage(disc, a, false)

		var err error
		for i := 0; i < 2; i++ {

			disc.UDPMsgConn.WriteTo(jsonAnn, addr)

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
		}
	}

	log.Println("/dis/ann", cnt, ok, fail)

	// if any interface is AP client ( 49.x but not 1) - attempt to connect AP
	//go gw.ensureConnectedUp(nil, nil)
}

var (
	selfRegister = errors.New("Self register")
	dup          = errors.New("Dup")
)
