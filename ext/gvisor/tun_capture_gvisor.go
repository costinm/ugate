package gvisor

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/costinm/ugate"

	"gvisor.dev/gvisor/pkg/context"
	"gvisor.dev/gvisor/pkg/sentry/socket/netstack"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/link/loopback"
	"gvisor.dev/gvisor/pkg/tcpip/link/sniffer"
	"gvisor.dev/gvisor/pkg/tcpip/network/arp"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/raw"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

// Intercept using a TUN and google netstack to parse TCP/UDP into streams.
// The connections are redirected to a capture.ProxyHandler
type GvisorTun struct {
	// The IP stack serving the tun. It intercepts all TCP connections.
	IPStack *stack.Stack

	DefUDP tcpip.Endpoint
	DefTCP tcpip.Endpoint

	udpPacketConn net.PacketConn

	// If set, will be used to handle accepted TCP connections and UDP packets.
	// Else the Listener interface is used.
	Handler    TUNHandler
	UDPHandler UDPHandler
}

// Used by the TUN interface
type UDPHandler interface {
	HandleUdp(dstAddr net.IP, dstPort uint16, localAddr net.IP, localPort uint16, data []byte)
}

// Interface implemented by uGate.
//
// Important: for android the system makes sure tun is the default route, but
// packets from the VPN app are excluded.
//
// On Linux we need a similar setup. This still requires iptables to mark
// packets from istio-proxy, and use 2 routing tables.
//
type TUNHandler interface {
	HandleTUN(conn net.Conn, target *net.TCPAddr, la *net.TCPAddr) error
}

// UdpWriter is the interface implemented by the GvisorTun, to send
// raw UDP packets back to the virtual interface
type UdpWriter interface {
	WriteTo(data []byte, dstAddr *net.UDPAddr, srcAddr *net.UDPAddr) (int, error)
}

/*

 Client:
	- tun app has access to real network - can send/receive to any host directly,
  -- may have real routable IPv4 and/or IPV6 address
  -- may be inside a mesh - only IPv6 link local communication with other nodes
  - regular apps have the default route set to the TUN device (directly or via rule).
	- tun_capture read all packets from regular apps, terminates TCP and receives UDP
  - the TCP can forward to real destination, or tunnel to some other node.
  - or it can tunnel all connections to it's VPN server, using a QUIC forwarder at TCP
   level.


 Server:
  - server operates on L7 streams only - originates TCP and UDP as client
  - client requests are muxed over h2 (or QUIC)
  - no TUN required, no masq !
  - skips the tunneled IP and TCP headers - metadata sent at start of stream
  - only the external IP/UDP/QUIC headers.

 Both:
  - each node can act as a server - forwarding streams either upstream or to nodes in
    same mesh
  - when acting as client, it can operate without TUN - forwarding TCP streams or UDP
   at L7.
  - tun capture requires VPN to be enabled, and transparently captures all TCP

 Alternatives:
  - tun_client captures all ip frames and sends them to VPN server
  - tun_server receives ip frames from clients, injects in local tun which does ipmasq
  - tun_server can also route to other clients directly, based on ip6

*/

/*
 Example android:
10: tun0: <POINTOPOINT,UP,LOWER_UP> mtu 1400 qdisc pfifo_fast state UNKNOWN qlen 500
    link/none
    inet 10.10.154.232/24 scope global tun0
    inet6 2001:470:1f04:429:4a46:48e5:ae34:9ae8/64 scope global
       valid_lft forever preferred_lft forever

ip route list table all

default via 10.1.10.1 dev wlan0  table wlan0  proto static
10.1.10.0/24 dev wlan0  table wlan0  proto static  scope link
default dev tun0  table tun0  proto static  scope link
10.1.10.0/24 dev wlan0  proto kernel  scope link  src 10.1.10.124
10.10.154.0/24 dev tun0  proto kernel  scope link  src 10.10.154.232
2001:470:1f04:429::/64 dev tun0  table tun0  proto kernel  metric 256
fe80::/64 dev tun0  table tun0  proto kernel  metric 256

ip rule show
0:	from all lookup local
10000:	from all fwmark 0xc0000/0xd0000 lookup legacy_system

11000:	from all iif tun0 lookup local_network

12000:	from all fwmark 0xc0066/0xcffff lookup tun0

### EXCLUDED: VPN process
12000:	from all fwmark 0x0/0x20000 uidrange 0-10115 lookup tun0
12000:	from all fwmark 0x0/0x20000 uidrange 10117-99999 lookup tun0

13000:	from all fwmark 0x10063/0x1ffff lookup local_network
13000:	from all fwmark 0x10064/0x1ffff lookup wlan0

13000:	from all fwmark 0x10066/0x1ffff uidrange 0-0 lookup tun0

13000:	from all fwmark 0x10066/0x1ffff uidrange 0-10115 lookup tun0
13000:	from all fwmark 0x10066/0x1ffff uidrange 10117-99999 lookup tun0

14000:	from all oif wlan0 lookup wlan0

14000:	from all oif tun0 uidrange 0-10115 lookup tun0
14000:	from all oif tun0 uidrange 10117-99999 lookup tun0

15000:	from all fwmark 0x0/0x10000 lookup legacy_system
16000:	from all fwmark 0x0/0x10000 lookup legacy_network
17000:	from all fwmark 0x0/0x10000 lookup local_network
19000:	from all fwmark 0x64/0x1ffff lookup wlan0
21000:	from all fwmark 0x66/0x1ffff lookup wlan0
22000:	from all fwmark 0x0/0xffff lookup wlan0
23000:	from all fwmark 0x0/0xffff uidrange 0-0 lookup main

32000:	from all unreachable

*/

/*
google.transport:
- transport_demuxer.go endpoints has a table of ports to endpoints

Life of packet:
-> NIC.DeliverNetworkPacket - will make a route - remote address/link addr, nexthop, netproto
-> ipv6.HandlePacket
-> NIC.DeliverTransportPacket
-- will first attempt nic.demux, then n.stac.demux deliverPacket
-- will look for an endpoint
-- packet added to the rcv linked list
-- waiter.dispatchToChannelHandlers()

RegisterTransportEndpoint -> with the stack transport dispatcher (nic.demux), on NICID
-- takes protocol, id - registers endpoint
-- for each net+transport protocol pair, one map based on 'id'
-- id== local port, remote port, local address, remote address
--


- a NIC is created with ID(int32), [name] and 'link endpoint ID' - which is a uint64 in the 'link endpoints'
static table. The LinkEndpoint if has MTU, caps, LinkAddress(MAC), WritePacket and Attach(NetworkDispatcher)
The NetworkDispatcher.DeliverNetworkPacket is also implemented by NIC

*/

var MTU = 9000

// NewTUNFD creates a gVisor stack on a TUN.
func NewTUNFD(fd io.ReadWriteCloser, handler TUNHandler, udpNat UDPHandler) UdpWriter {
	var linkID stack.LinkEndpoint

	useFD := os.Getenv("CHANNEL_LINK") == ""
	if f, ok := fd.(*os.File); ok && useFD {
		// Bugs - after some time it stops reading.
		// Workaround is to use the regular read with a patch.
		ep, err := fdbased.New(&fdbased.Options{
			MTU: uint32(MTU),
			FDs: []int{int(f.Fd())},
			// Readv = slowers, supported on TUN
			// RecvMMsg = default for gvisor. Only supported for sockets
			// PacketMMap = AF_PACKET, for veth
			PacketDispatchMode: fdbased.RecvMMsg,
			ClosedFunc: func(t tcpip.Error) {
				log.Println("XXX GVISOR CLOSE ", t)
			},
		})
		if err != nil {
			log.Println("Link err", err)
		}
		linkID = ep
		t := NewGvisorTunCapture(&linkID, handler, udpNat, false)

		return t
	} else {
		log.Println("Using channel based link")
		ep := channel.New(1024, uint32(MTU), "")
		linkID = ep

		t := NewGvisorTunCapture(&linkID, handler, udpNat, false)

		go func() {
			m1 := make([]byte, MTU)
			for {
				n, err := fd.Read(m1)
				if err != nil {
					log.Println("NIC read err", err)
					continue
				}
				b := buffer.NewViewFromBytes(m1[0:n])
				pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
					Data: b.ToVectorisedView(),
				})
				ep.InjectInbound(ipv4.ProtocolNumber, pkt)
			}
		}()

		go func() {
			m1 := make([]byte, MTU)
			ctx := context.Background()
			for {
				// Read is non-blocking
				pi, err := ep.ReadContext(ctx)
				if !err {
					continue
				}
				if pi.Pkt == nil {
					continue
				}

				// ToView also returns the first view - but allocates a buffer if multiple
				// PullUp allocates too

				// TODO: track how many views in each packet

				// TODO: reuse a buffer
				vv := pi.Pkt.Views()
				if len(vv) == 1 {
					fd.Write(vv[0])
				} else {
					n := 0
					for _, v := range vv {
						copy(m1[n:], v)
						n += v.Size()
					}

					fd.Write(m1[0:n])
				}
			}
		}()
		return t
	}

}

//type mymatch struct {
//
//}
//
//func (*mymatch) Name() string {
//	return "my"
//}
//
//func (*mymatch) 	Match(hook stack.Hook, pkt *stack.PacketBuffer, interfaceName string) (matches bool, hotdrop bool) {
//	tcpHeader := header.TCP(pkt.TransportHeader().View())
//	if tcpHeader.DestinationPort() == 5201 {
//		return true, false
//	}
//	return false , false
//}

// NewTunCapture creates an in-process tcp stack, backed by an tun-like network interface.
// All TCP streams initiated on the tun or localhost will be captured.
func NewGvisorTunCapture(ep *stack.LinkEndpoint, handler TUNHandler, udpNat ugate.UDPHandler, snif bool) *GvisorTun {
	t := &GvisorTun{
		Handler:    handler,
		UDPHandler: udpNat,
	}

	netProtos := []stack.NetworkProtocolFactory{
		ipv4.NewProtocol,
		ipv6.NewProtocol,
		arp.NewProtocol}
	transProtos := []stack.TransportProtocolFactory{
		tcp.NewProtocol,
		udp.NewProtocol,
		icmp.NewProtocol4,
		icmp.NewProtocol6,
	}

	//ipt := netfilter.DefaultLinuxTables()
	//// 3 tables for ip4
	//if false {
	//	natt := ipt.GetTable(stack.NATID, false)
	//	//// To trigger modified = true
	//	ipt.ReplaceTable(stack.NATID, natt, false)
	//	//// Default has 5 rules.
	//	//// HAck !!!!
	//	natt.Rules[0].Target = &stack.RedirectTarget{
	//		Port:            5201,
	//		NetworkProtocol: ipv4.ProtocolNumber}
	//	natt.Rules[0].Filter = stack.IPHeaderFilter{
	//		Protocol:      tcp.ProtocolNumber,
	//		CheckProtocol: true,
	//	}
	//	//// Can only create matcher using unmarshal
	//	natt.Rules[0].Matchers = []stack.Matcher{
	//		&mymatch{},
	//	}
	//}

	t.IPStack = stack.New(stack.Options{
		NetworkProtocols:   netProtos,
		TransportProtocols: transProtos,
		//Clock:              clock,
		Stats:       netstack.Metrics,
		HandleLocal: false, // accept from other nics
		// Enable raw sockets for users with sufficient
		// privileges.
		RawFactory: raw.EndpointFactory{},
		//UniqueID:   uniqueID,
		//IPTables:   ipt,
	})

	loopbackLinkID := loopback.New()
	if true || snif {
		loopbackLinkID = sniffer.New(loopbackLinkID)
	}
	t.IPStack.CreateNIC(1, loopbackLinkID)

	addr1 := "\x7f\x00\x00\x01"
	if err := t.IPStack.AddAddress(1, ipv4.ProtocolNumber,
		tcpip.Address(addr1)); err != nil {
		log.Print("Can't add address", err)
		return t
	}
	if err := t.IPStack.AddAddress(1, ipv6.ProtocolNumber, tcpip.Address(net.IPv6loopback)); err != nil {
		log.Print("Can't add IP6 address", err)
		return t
	}

	ep1 := *ep

	// NIC 2 - IP4, IP6 - the TUN device
	t.IPStack.CreateNIC(2, ep1)

	addr2 := "\x0a\x0b\x00\x02"
	if err := t.IPStack.AddAddress(2, ipv4.ProtocolNumber, tcpip.Address(addr2)); err != nil {
		log.Print("Can't add address", err)
		return t
	}
	addr3, _ := net.ResolveIPAddr("ip", "fd::1:2")
	if err := t.IPStack.AddAddress(2, ipv6.ProtocolNumber, tcpip.Address(addr3.IP)); err != nil {
		log.Print("Can't add address", err)
		return t
	}

	t.IPStack.SetPromiscuousMode(2, true)
	t.IPStack.SetSpoofing(2, true)

	sn, _ := tcpip.NewSubnet(tcpip.Address("\x00"), tcpip.AddressMask("\x00"))
	t.IPStack.AddRoute(tcpip.Route{NIC: 2, Destination: sn})

	sn, _ = tcpip.NewSubnet(tcpip.Address("\x00"), tcpip.AddressMask("\x00"))
	//t.IPStack.AddSubnet(2, ipv6.ProtocolNumber, sn)
	t.IPStack.AddRoute(tcpip.Route{NIC: 2, Destination: sn})

	gsetRouteTable(t.IPStack, ep != nil)

	//epp := newEpProxy()
	go t.DefTcpServer(handler) //echo)

	go t.DefTcp6Server() //echo)

	go t.defUdpServer()
	t.defUdp6Server()

	return t
}

func (nt *GvisorTun) WriteTo(data []byte, dst *net.UDPAddr, src *net.UDPAddr) (int, error) {
	addrb := []byte(dst.IP)
	srcaddrb := []byte(src.IP)
	// TODO: how about from ?
	// TODO: do we need to make a copy ? netstack passes ownership, we may reuse buffers
	n, err := nt.DefUDP.Write(bytes.NewBuffer(data),
		tcpip.WriteOptions{
			To: &tcpip.FullAddress{
				Port: uint16(dst.Port),
				Addr: tcpip.Address(addrb),
			},
			// TODO(costin): PATCH
			From: &tcpip.FullAddress{
				Port: uint16(src.Port),
				Addr: tcpip.Address(srcaddrb),
			},
		})
	if err != nil {
		return 0, errors.New(err.String())
	}
	return int(n), nil
}

func (nt *GvisorTun) defUdpServer() error {
	// Like a socket
	var wq waiter.Queue
	ep, err := nt.IPStack.NewEndpoint(udp.ProtocolNumber, ipv4.ProtocolNumber, &wq)
	if err != nil {
		return errors.New(err.String())
	}
	nt.DefUDP = ep

	// No address - listen on all
	err = ep.Bind(tcpip.FullAddress{
		//Addr: "\x01", - error
		//Addr: "\x00\x00\x00\x00",
		//Port: 2000,
		Port: 0xffff,
		//Port: 15001,
	})
	if err != nil {
		ep.Close()
		return errors.New(err.String())
	}
	ep.SocketOptions().SetReceiveOriginalDstAddress(true)

	we, ch := waiter.NewChannelEntry(nil)
	wq.EventRegister(&we, waiter.EventIn)

	ro := tcpip.ReadOptions{NeedRemoteAddr: true}
	go func() {
		for {
			// Will have the peer address
			//ep.SetSockOpt()
			// StartListener is send address. Control should include the dest addr ( for raw )
			bb := &bytes.Buffer{}
			rr, err := ep.Read(bb, ro)
			//v, _, err := ep.(UdpLocalReader).ReadLocal(&add)
			if _, ok := err.(*tcpip.ErrWouldBlock); ok {
				select {
				case <-ch:
					continue
				}
			}

			// TODO: add back full address for UDP
			if nt.UDPHandler != nil {
				nt.UDPHandler.HandleUdp(net.IP(rr.ControlMessages.OriginalDstAddress.Addr),
					rr.ControlMessages.OriginalDstAddress.Port,
					net.IP(rr.RemoteAddr.Addr), rr.RemoteAddr.Port,
					bb.Bytes())
			}
		}
	}()

	return nil
}

func (nt *GvisorTun) defUdp6Server() error {
	// Like a socket
	//var wq waiter.Queue
	//
	//ep6, err := nt.IPStack.NewEndpoint(udp.ProtocolNumber, ipv6.ProtocolNumber, &wq)
	//if err != nil {
	//	return errors.New(err.String())
	//}
	//err = ep6.Bind(tcpip.FullAddress{
	//	//Addr: "\x01", - error
	//	Addr: tcpip.Address(net.IPv6loopback),
	//	//Port: 2000,
	//	Port: 0xffff,
	//	NIC:  2,
	//}, nil)
	//if err != nil {
	//	ep6.Close()
	//	return errors.New(err.String())
	//}
	//nt.IPStack.Capture(ipv6.ProtocolNumber, udp.ProtocolNumber, ep6.(stack.TransportEndpoint))
	//
	//we, ch := waiter.NewChannelEntry(nil)
	//wq.EventRegister(&we, waiter.EventIn)
	////defer wq.EventUnregister(&we)
	//
	//go func() {
	//	for {
	//		// Will have the peer address
	//		add := tcpip.DoubleAddress{}
	//		//ep.SetSockOpt()
	//		v, _, err := ep6.(UdpLocalReader).ReadLocal(&add)
	//		if err == tcpip.ErrWouldBlock {
	//			select {
	//			case <-ch:
	//				continue
	//			}
	//		}
	//
	//		la := net.IP([]byte(add.LocalAddr))
	//		//if la.To4() == nil {
	//		//	log.Print("IP6 ", la)
	//		//}
	//		if add.LocalAddr[0] == 0xff {
	//			continue
	//		}
	//
	//		if nt.UDPHandler != nil {
	//			nt.UDPHandler.HandleUdp(la, add.LocalPort,
	//				net.IP([]byte(add.FullAddress.Addr)), add.FullAddress.Port,
	//				v)
	//		}
	//
	//	}
	//}()

	return nil
}

//var (
//	Dump = false
//)

func (nt *GvisorTun) DefTcpServer(handler TUNHandler) {
	var wq waiter.Queue
	// No address - listen on all
	//err = ep.Bind(tcpip.FullAddress{
	//	Port: 0xffff,
	//}, nil) // reserves port

	//if err != nil {
	//	ep.Close()
	//	return nil, wq, errors.New(err.String())
	//}
	// MISSING ACCEPT
	//ep, _ := nt.IPStack.NewRawEndpoint( tcp.ProtocolNumber,ipv4.ProtocolNumber, &wq, false)
	ep, _ := nt.IPStack.NewEndpoint(tcp.ProtocolNumber, ipv4.ProtocolNumber, &wq)
	ep.Bind(tcpip.FullAddress{Port: 0xffff})
	//ep.Bind(tcpip.FullAddress{Port: 5201})
	if err := ep.Listen(100); err != nil { // calls Register
		ep.Close()
		return
	}

	tl := gonet.NewTCPListener(nt.IPStack, &wq, ep)
	for {
		c, err := tl.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		ra := c.LocalAddr().(*net.TCPAddr)
		la := c.RemoteAddr().(*net.TCPAddr)
		go handler.HandleTUN(c, ra, la)
	}
	//we, listenCh := waiter.NewChannelEntry(nil)
	//wq.EventRegister(&we, waiter.EventIn)
	//
	//// receive TCP packets on port
	//go func() {
	//	defer wq.EventUnregister(&we)
	//	for {
	//		epin, wqin, err := ep.Accept()
	//		if err != nil {
	//			if err == tcpip.ErrWouldBlock {
	//				<-listenCh
	//				continue
	//			}
	//			log.Println("Unexpected accept error")
	//		}
	//		if Dump {
	//			add, _ := epin.GetRemoteAddress()
	//			ladd, _ := epin.GetLocalAddress()
	//			log.Printf("TUN: Accepted %v %v", ladd, add)
	//		}
	//
	//		conn := gonet.NewConn(wqin, epin)
	//		go func() {
	//			err := handler.HandleTUN(conn)
	//			if err != nil {
	//				return
	//			}
	//		}()
	//
	//	}
	//}()
}

func (nt *GvisorTun) DefTcp6Server() {
	//var wq waiter.Queue
	//ep, err := nt.IPStack.NewEndpoint(tcp.ProtocolNumber, ipv6.ProtocolNumber, &wq)
	//if err != nil {
	//	return nil, wq, errors.New(err.String())
	//}
	//
	//// No address - listen on all
	//err = ep.Bind(tcpip.FullAddress{
	//	Addr: tcpip.Address(net.IPv6loopback),
	//	Port: 0xffff,
	//	NIC:  2,
	//}, nil) // reserves port
	//if err != nil {
	//	ep.Close()
	//	return nil, wq, errors.New(err.String())
	//}
	//nt.IPStack.Capture(ipv6.ProtocolNumber, tcp.ProtocolNumber, ep.(stack.TransportEndpoint))
	//
	//if err := ep.Listen(10); err != nil { // calls Register
	//	ep.Close()
	//	return nil, wq, errors.New(err.String())
	//}
	//
	//we, listenCh := waiter.NewChannelEntry(nil)
	//wq.EventRegister(&we, waiter.EventIn)
	//
	//// receive TCP packets on port
	//go func() {
	//	defer wq.EventUnregister(&we)
	//	for {
	//		epin, wqin, err := ep.Accept()
	//		if err != nil {
	//			if err == tcpip.ErrWouldBlock {
	//				<-listenCh
	//				continue
	//			}
	//			log.Println("Unexpected accept error")
	//		}
	//		if Dump {
	//			add, _ := epin.GetRemoteAddress()
	//			ladd, _ := epin.GetLocalAddress()
	//			log.Printf("TUN: Accepted %v %v", ladd, add)
	//		}
	//
	//		conn := gonet.NewConn(wqin, epin)
	//		go func() {
	//			err := nt.Handler.HandleTUN(conn)
	//			if err != nil {
	//				return
	//			}
	//		}()
	//
	//	}
	//}()
	//
	//return ep, wq, nil
}

func ga2na(address tcpip.Address) net.IP {
	ab := []byte(address)
	return net.IP(ab)
}

func sn(net, mask string) tcpip.Subnet {
	r, _ := tcpip.NewSubnet(tcpip.Address([]byte(net)), tcpip.AddressMask([]byte(mask)))
	return r
}

func gsetRouteTable(ipstack *stack.Stack, real bool) {
	ipstack.SetRouteTable([]tcpip.Route{
		{
			Destination: sn("\x7f\x00\x00\x00", "\xff\x00\x00\x00"),
			Gateway:     "",
			NIC:         1,
		},
		{ // 10.12.0.2 - IP of the tun
			Destination: sn("\x0a\x0c\x00\x02", "\xff\xff\xff\xff"),
			Gateway:     "",
			NIC:         2,
		},
		{ // 10.12.0.0 - routed to the tun
			Destination: sn("\x0a\x0c\x00\x00", "\xff\xff\x00\x00"),
			Gateway:     "",
			NIC:         2,
		},
		{
			Destination: sn("\x00\x00\x00\x00", "\x00\x00\x00\x00"),
			Gateway:     "",
			NIC:         2,
		},
		{
			Destination: sn(string(net.IPv6loopback), strings.Repeat("\xff", 16)),
			Gateway:     "",
			NIC:         1,
		},
		{
			Destination: sn(strings.Repeat("\x00", 16), strings.Repeat("\x00", 16)),
			Gateway:     "",
			NIC:         2,
		},
	})
}

/*
	Terms:
 - netstack - the network stack implementation
 - nic - virtual interface
 -- route table
 -- address

 - packet injected and sent by link - dmtun (but doesn't work android) or channel based

 - View - slice of buffer, TrimFront, CapLength,
 -
*/
