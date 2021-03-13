/*
 * Copyright (c) 2016 Felipe Cavalcanti <fjfcavalcanti@gmail.com>
 * Author: Felipe Cavalcanti <fjfcavalcanti@gmail.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package udp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/costinm/ugate"
)

// Handles captured UDP packets.
// DNS is handled using the special package.

// If a VPN is present - currently no UDP proxy.
// Without VPN - a NAT equivalent is used.

// TURN is the protocol for rely. Supports UDP-over TLS-TCP
// "Allocation" is created - client/server IP and port
// 2 consecutive ports allocated?
// 49152 - 65535 range

// TURN and STUN: default 3478, and 5349 for TLS

// UDP NAT
// - STUN or ICE -to determine the public IP and port
// - TTL is 10s of sec to minutes
// - symmetric - restrict port like in tcp

// most NATs keep alive require send from inside
// 'often 30 s' (TCP is often 15 min)

// ALG (application layer gateway) - for example for SIP
//

// Existing proxies:

// fateier/frp
// -- most complete
// -- similar

// - go get github.com/litl/shuttle
// -- admin interface, dynamic config
// -- bench
// -- service.go:runUDP() - appears to be one-way
// -- no buffer reuse

// - go get github.com/crosbymichael/proxy
// -- setRLimit in go
// -- rcrowley/go-metrics - multiple clients, richer than prom
// -- EPIPE check
// -- no UDP yet

// - go get github.com/felipejfc/udpx
// -- udp only
// -- udpproxy.go is the main class - timeouts, proxy
// -- no buffer reuse, extra channel for listener packets.
// -- simplest
// -- zap dep

// Interesting traffic:
// - o-o.myaddr.l.google.com. [o-o.myaddr.l.google.com.	60	IN	TXT	"73.158.64.15"]

var (
	DumpUdp = false
)

type UdpRelay struct {

	// The 4-tuple is used as a key
	ClientIP   net.IP
	ClientPort uint16

	ServerIP   net.IP
	ServerPort uint16

	// Relay ports
	ClientRelayPort uint16
	ServerRelayPort uint16
}

func (u *UdpRelay) relayLoop() {

}

var (
	bufferPoolUdp = sync.Pool{New: func() interface{} {
		return make([]byte, 0, 1600)
	}}
)

// Represents on UDP 'nat' connection.
// Currently full cone, i.e. one local port per NAT.
type UdpNat struct {
	ugate.Stream
	// bound to a local port (on the real network).
	UDP *net.UDPConn

	Closed    bool
	LocalPort int

	LastRemoteIP    net.IP
	LastsRemotePort uint16
}


type UDPGate struct {

	// NAT
	udpLock   sync.RWMutex
	ActiveUdp map[string]*UdpNat
	AllUdpCon map[string]*ugate.HostStats

	// UDP
	// Capture return - sends packets back to client app.
	// This is typically a netstack or TProxy
	UDPWriter ugate.UdpWriter


	DNS ugate.UDPHandler
	HostResolver ugate.HostResolver
	//DNS *dmdns.DmDns

	// Timeout for UDP sockets. Default 60 sec.
	ConnTimeout time.Duration
}

func NewUDPGate(dnsHandler ugate.UDPHandler, hostRes ugate.HostResolver) *UDPGate {
	return &UDPGate{
		DNS: dnsHandler,
		HostResolver: hostRes,
		ConnTimeout: 60 * time.Second,
		ActiveUdp:   map[string]*UdpNat{},
		AllUdpCon:   map[string]*ugate.HostStats{},
	}
}
// http debug/ui

func (gw *UDPGate) InitMux(mux *http.ServeMux) {
	mux.HandleFunc("/dmesh/udpa", gw.HttpUDPNat)
	mux.HandleFunc("/dmesh/udp", gw.HttpAllUDP)
}

func (gw *UDPGate) HttpAllUDP(w http.ResponseWriter, r *http.Request) {
	gw.udpLock.RLock()
	defer gw.udpLock.RUnlock()
	json.NewEncoder(w).Encode(gw.AllUdpCon)
}

func (gw *UDPGate) HttpUDPNat(w http.ResponseWriter, r *http.Request) {
	gw.udpLock.RLock()
	defer gw.udpLock.RUnlock()
	json.NewEncoder(w).Encode(gw.ActiveUdp)
}




//// Server side of 'UDP-over-H2+QUIC'.
//// 3 modes:
//// - /udp/ - allocates a pair of ports. No encryption.
//// - TODO: /udptcp/ UDP encapsulated in H2 stream.
//// - TODO: /udps/ - negotiate an encryption key. Use a single UDP port. Encrypt and header.
//func (gw *Gateway) HTTPTunnelUDP(w http.ResponseWriter, r *http.Request) {
//
//	// remote address and port
//
//}

// NAT will open one port per clientIP+port, and forward back to the local app.
// This is the forward loop.ud
func remoteConnectionReadLoop(gw *UDPGate, localAddr *net.UDPAddr, upstreamConn *net.UDPConn, udpN *UdpNat) {
	if DumpUdp {
		log.Println("Starting read loop for ", localAddr)
	}
	clientAddrString := localAddr.String()
	for {
		// TODO: reuse, detect MTU. Need to account for netstack buffer ownership
		buffer := bufferPoolUdp.Get().([]byte)
		// upstreamConn is a UDP Listener bound to a random port, receiving messages
		// from the remote app (or any other app in case of STUN)
		size, srcAddr, err := upstreamConn.ReadFromUDP(buffer[0:cap(buffer)])
		if err != nil {
			if udpN.Closed {
				return
			}
			log.Println("UDP: close read loop for ", localAddr, err)
			upstreamConn.Close()

			gw.udpLock.Lock()
			delete(gw.ActiveUdp, clientAddrString)
			gw.udpLock.Unlock()
			return
		}
		udpN.LastRead = time.Now()
		udpN.RcvdPackets++
		udpN.RcvdBytes += size

		udpN.LastRemoteIP = srcAddr.IP
		udpN.LastsRemotePort = uint16(srcAddr.Port)

		// TODO: for android dmesh, we may need to take zone into account.
		if gw.UDPWriter != nil {
			if DumpUdp {
				log.Println("UDP Res: ", srcAddr, "->", localAddr)
			}
			n, err := gw.UDPWriter.WriteTo(buffer[:size], localAddr, srcAddr)
			if DumpUdp {
				log.Println("UDP Res DPME: ", srcAddr, "->", localAddr, n, err)
			}
		} else {
			if DumpUdp {
				log.Println("UDP direct RES: ", srcAddr, "->", localAddr)
			}
			upstreamConn.WriteTo(buffer[:size], localAddr)
			bufferPoolUdp.Put(buffer)
		}

		//p.upstreamMessageChannel <- packet{
		//	src:  localAddr,
		//	data: buffer[:size],
		//}
	}
}

//func (p *Gateway) handlerRemotePackets() {
//	for pa := range p.upstreamMessageChannel {
//		p.UDPWriter.WriteTo(pa.data, pa.src)
//	}
//}

// HandleUDP is processing a captured UDP packet. It can be captured by iptables TPROXY or
// netstack TUN.
func (gw *UDPGate) HandleUdp(dstAddr net.IP, dstPort uint16,
		localAddr net.IP, localPort uint16, data []byte) {
	if dstPort == 1900 {
		return
	}
	if dstPort == 5353 {
		return
	}
	if dstPort == 123 {
		return
	}
	if dstAddr[0] == 230 {
		return
	}

	if dstPort == 53 && gw.DNS != nil {
		gw.DNS.HandleUdp(dstAddr, dstPort, localAddr, localPort, data)
		return
	}
	src := &net.UDPAddr{Port: int(localPort), IP: localAddr}

	//if gw.Vpn != "" {
	// TODO: implement UDP-over-H2/QUIC
	//return
	//}

	packetSourceString := src.String()

	gw.udpLock.RLock()
	conn, found := gw.ActiveUdp[packetSourceString]
	gw.udpLock.RUnlock()

	if !found {
		// port 0
		// TODO: attempt to use the localPort first
		udpCon, err := net.ListenUDP("udp", &net.UDPAddr{
			Port: 0,
		})

		if err != nil {
			log.Println("udp proxy failed to listen", err)
			FreeIdleSockets(gw)
			return
		}

		conn = &UdpNat{
			UDP: udpCon,
		}
		conn.DestAddr = &net.TCPAddr{
			IP: dstAddr,
			Port: int(dstPort),
		}
		if gw.HostResolver != nil {
			dns, _ := gw.HostResolver.HostByAddr(dstAddr.String())
			conn.DestDNS = fmt.Sprintf("%s:%d", dns, dstPort)
		} else {
			conn.DestDNS = fmt.Sprintf("%s:%d", dstAddr.String(), dstPort)
		}

		conn.LocalPort = udpCon.LocalAddr().(*net.UDPAddr).Port
		gw.udpLock.Lock()
		gw.ActiveUdp[packetSourceString] = conn
		gw.udpLock.Unlock()

		// all packets on the source port will be sent back to localAddr:localPort
		go remoteConnectionReadLoop(gw, src, udpCon, conn)

	}

	dst := &net.UDPAddr{Port: int(dstPort), IP: dstAddr}
	n, err := conn.UDP.WriteTo(data, dst)

	conn.LastWrite = time.Now()
	conn.SentPackets++
	conn.SentBytes += len(data)
	conn.Open = time.Now()

	if gw.HostResolver != nil {
		dns, _ := gw.HostResolver.HostByAddr(dstAddr.String())
		conn.DestDNS = fmt.Sprintf("%s:%d", dns, dstPort)
	} else {
		conn.DestDNS = fmt.Sprintf("%s:%d", dstAddr.String(), dstPort)
	}

	if DumpUdp {
		if found {
			log.Println("UDP OFW ", src, "->", dst, n)
		} else {
			log.Println("UDP open ", src, "->", conn.UDP.LocalAddr(), "->", dst, n, err)
		}
		log.Println("UDP open ", src, "->", conn.UDP.LocalAddr(), "->", dst, n)
	}
}

// Called on the periodic cleanup thread (~60sec), or if too many sockets open.
// Will update udp stats. Default UDP timeout to 60 sec.
func FreeIdleSockets(gw *UDPGate) {

	var clientsToTimeout []string

	gw.udpLock.Lock()
	active := 0
	t0 := time.Now()
	for client, remote := range gw.ActiveUdp {
		if t0.Sub(remote.LastWrite) > gw.ConnTimeout {
			log.Printf("UDPC: %s:%d rcv=%d/%d snd=%d/%d ac=%v ra=%v op=%v lr=%s:%d la=%s %s",
				remote.DestAddr.IP, remote.DestAddr.Port,
				remote.RcvdPackets, remote.RcvdBytes,
				remote.SentPackets, remote.SentBytes,
				time.Since(remote.LastWrite), time.Since(remote.LastRead), time.Since(remote.Open),
				remote.LastRemoteIP, remote.LastsRemotePort, remote.UDP.LocalAddr(), client)
			remote.Closed = true
			clientsToTimeout = append(clientsToTimeout, client)

			hs, f := gw.AllUdpCon[remote.Dest]
			if !f {
				hs = &ugate.HostStats{Open: time.Now()}
				gw.AllUdpCon[remote.Dest] = hs
			}
			hs.Last = remote.LastRead
			hs.SentPackets += remote.SentPackets
			hs.SentBytes += remote.SentBytes
			hs.RcvdPackets += remote.RcvdPackets
			hs.RcvdBytes += remote.RcvdBytes

			hs.LastLatency = hs.Last.Sub(remote.Open)
			hs.LastBPS = int(int64(hs.RcvdBytes) * 1000000000 / hs.LastLatency.Nanoseconds())
			hs.Count++
		} else {
			//log.Printf("UDPC: active %v", remote)
			active++
		}
	}
	gw.udpLock.Unlock()

	if active > 0 || len(clientsToTimeout) > 0 {
		log.Printf("Closing %d active %d", len(clientsToTimeout), active)
	} else {
		return
	}
	gw.udpLock.Lock()
	for _, client := range clientsToTimeout {
		gw.ActiveUdp[client].UDP.Close()
		delete(gw.ActiveUdp, client)
	}
	gw.udpLock.Unlock()
}

func CloseUdp(gw *UDPGate) {
	gw.udpLock.Lock()
	//gw.closed = true
	for _, conn := range gw.ActiveUdp {
		conn.UDP.Close()
	}
	gw.udpLock.Unlock()
}
