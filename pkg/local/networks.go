package local

import (
	"bytes"
	"log"
	"net"
	"strings"
	"time"
)

// Periodic refresh and registration.
func (gw *LLDiscovery) PeriodicThread() error {
	// TODO: dynamically adjust the timer
	// TODO: CON and AP events should be sufficient - check if Refresh is picking anything
	// new.
	ticker := time.NewTicker(120 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				gw.RefreshNetworks()
				// TODO: keepalive or register on VPN, if ActiveInterface (may use different timer)
			case <-quit:
				return
			}
		}
	}()

	ticker2 := time.NewTicker(15 * time.Minute)
	quit2 := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker2.C:
				// TODO
			case <-quit2:
				return
			}
		}
	}()

	gw.RefreshNetworks()
	return nil
}

// TODO: make sure it is called when android changes interfaces (AP, CON events)

// RefreshNetworks will update the list of ActiveInterface networks, and ensure each has a listener.
// Local communication uses the link-local address. If the interface
// is connected to an Android AP, it uses a link-local multicast address instead.
//
// - Called from android using "r" message, on connectivity changes
// - Also called from android at startup and property changes ( "P" - properties ).
// - 15-min thread on link local
func (gw *LLDiscovery) RefreshNetworks() {
	refreshMutex.Lock()
	defer refreshMutex.Unlock()

	t0 := time.Now()
	newAct, err := ActiveNetworks(gw)

	if err != nil {
		log.Println("Error getting active networks ", err)
		return
	}
	unchanged := map[string]*ActiveInterface{}

	gw.activeMutex.Lock()
	defer gw.activeMutex.Unlock()

	changed := false

	// True if any of the interfaces is an Android AP
	hasAp := false

	// For disconnected networks - close the sockets and stop listening
	for nname, existing := range gw.ActiveInterfaces {
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

		changed = true
		// No longer ActiveInterface. The socket is probably closed already
		log.Println("MCDirect: Interface no longer active ", existing)
		if existing.AndroidAP {
			gw.ApStopTime = time.Now()
			gw.ApRunTime = gw.ApRunTime + gw.ApStopTime.Sub(gw.ApStartTime)
		}
		if existing.unicastUdpServer != nil {
			existing.unicastUdpServer.Close()
		}
		if existing.unicastUdpServer4 != nil {
			existing.unicastUdpServer4.Close()
		}
		//if existing.tcpListener != nil {
		//	existing.tcpListener.Close()
		//}
		//if existing.multicastUdpServer != nil {
		//	existing.multicastUdpServer.Close()
		//}
		if existing.multicastRegisterConn != nil {
			existing.multicastRegisterConn.Close()
		}
		if existing.multicastRegisterConn2 != nil {
			existing.multicastRegisterConn2.Close()
		}
	}

	names := []string{}
	// Already ActiveInterface - copy over the socket
	for _, a := range newAct {
		//if /*a.iface.Name == "p2p0" && */ a.iface.Name != lm.Register.Gateway.ActiveP2P {
		//	// Otherwise 'address in use' attempting to listen on android AP
		//	// New devices use p2p0. Older have p2p0 and another one
		//	continue
		//}
		if a.Port == 0 {
			a.Port = gw.baseListenPort
		}
		if a.AndroidAP {
			hasAp = true
		}
		names = append(names, a.Name)

		if a.unicastUdpServer == nil && a.IP6LL != nil {
			changed = true
			port, ucListener := gw.listen6(gw.baseListenPort, a.IP6LL, a)
			if ucListener == nil {
				log.Println("MCDirect: Failed to start unicast server ", err)
			} else {
				a.Port = port
				a.unicastUdpServer = ucListener
			}
		}

		if a.unicastUdpServer4 == nil && a.IP4 != nil {
			changed = true
			port4, ucListener4 := gw.listen4(gw.baseListenPort, a.IP4, a)
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
				Port: gw.mcPort,
				Zone: a.iface.Name,
			}
			mc4APUDPAddr := &net.UDPAddr{
				IP:   MulticastDiscoveryIP4,
				Port: gw.mcPort,
				Zone: a.iface.Name,
			}
			if a.AndroidAP {
				gw.ApStartTime = time.Now()
			}
			if a.IP6LL != nil {
				m, err := net.ListenMulticastUDP("udp6", a.iface, mc6APUDPAddr)
				if err != nil {
					log.Println("MCDirect: Failed to start multicast6 ", a, err)
				} else {
					//log.Println("MCDirect: MASTER: ", a.IP6LL, a.iface.Name, mc6APUDPAddr, a.IP4, a.AndroidAP)
					a.multicastRegisterConn = m
					a := a
					go gw.multicastReaderThread(m, a)
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
					go gw.multicastReaderThread(m2, a)
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

	// TODO: if !hasAp - shut down the multicast register listener on the interface.
	// only the ones acting as AP need to discover peers.

	gw.ActiveInterfaces = newAct
	t1 := time.Now()
	// TODO: if any interface IP changed, reset all direct nodes/gateways
	// (at least on the same zone that changed)
	if changed {
		go gw.AnnounceMulticast()

		log.Println("MCDirect: RefreshNetworks", time.Since(t0), time.Since(t1), names)
	}

	//go gw.OnLocalNetworkFunc(nil, nil, gw.isMeshClient(), false)

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
		log.Println("failed to get intefaces", err)
		return nil, err
	}
	gw.ActiveP2P = ""

	for _, ifi := range ifaces {
		ifi := ifi
		if ifi.Flags&net.FlagUp == 0 {
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

func findActive(tofind *ActiveInterface, in map[string]*ActiveInterface) *ActiveInterface {
	if tofind == nil || in == nil {
		return nil
	}
	a, _ := in[tofind.iface.Name]
	return a
}
