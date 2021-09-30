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
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

// TODO: For TUN capture, we can process the UDP packet directly, without going through the stack.
// Need to evaluate if TUN can avoid the perf issues with regular UDP
//
// TODO: use multiple cores, pin routines to threads and threads to cores, bind multiple times.
//
// TODO: detection, dispatch to WebRTC/STUN/Quic
//
// TODO: Copy from quic the code to setReadBuffer.
//
// TODO: wrap UDP connection, exposing OOBCapablePacketConn and SetReadBuffer



// Handles captured UDP packets (for TUN and TProxy) and accepted sockets.
//
// For port 53, DNS is handled using the special DNS package.

// TURN is the protocol for rely. Supports UDP-over TLS-TCP
// "Allocation" is created - client/server IP and port
// 2 consecutive ports allocated?
// 49152 - 65535 range

// TURN and STUN: default 3478, and 5349 for TLS

// UDP NAT
//
// - STUN or ICE -to determine the public IP and port
// - TTL is 10s of sec to minutes
// - symmetric - restrict port like in tcp

// most NATs keep alive require send from inside
// 'often 30 s' (TCP is often 15 min)

// ALG (application layer gateway) - for example for SIP

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


const DumpUdp = true


var (
	bufferPoolUdp = sync.Pool{New: func() interface{} {
		return make([]byte, 0, 9000)
	}}
)

// Represents on UDP 'nat' connection or association.
//
// Currently full cone, i.e. one local port per NAT - max 30k
// This should be sufficient for local capture and small p2p nets.
// In the mesh, UDP should be encapsulated in WebRTC or quic.
type UdpNat struct {
	ugate.Stats

	// External address - string
	Dest string
	// External address
	DestAddr *net.UDPAddr

	//ugate.Conn
	// bound to a local port (on the real network).
	UDP *net.UDPConn

	Closed bool

	// For captured traffic / NAT
	LocalIP   net.IP
	LocalPort int

	LastRemoteIP    net.IP
	LastsRemotePort uint16
	ReverseSrcAddr  *net.UDPAddr
}

// Capture return - sends packets back to client app.
// This is typically a netstack or TProxy
var TransparentUDPWriter ugate.UdpWriter

type UDPGate struct {
	cfg *ugatesvc.UGate

	// NAT
	udpLock   sync.RWMutex
	ActiveUdp map[string]*UdpNat

	AllUdpCon map[string]*ugate.HostStats

	// UDP
	// Capture return - sends packets back to client app.
	// This is typically a netstack or TProxy
	TransparentUDPWriter ugate.UdpWriter

	// Timeout for UDP sockets. Default 60 sec.
	ConnTimeout time.Duration
}

func New(ug *ugatesvc.UGate) *UDPGate {
	udpg := &UDPGate{
		cfg:         ug,
		ConnTimeout: 60 * time.Second,
		ActiveUdp:   map[string]*UdpNat{},
		AllUdpCon:   map[string]*ugate.HostStats{},
	}
	if ug.Mux != nil {
		udpg.InitMux(ug.Mux)
	}
	ug.UDPHandler = udpg
	for k, l := range ug.Config.Listeners {
		if strings.HasPrefix(k, "udp://") {
			l.Address = k
			udpg.Listener(l)
			log.Println("uGate: listen UDP ", l.Address, l.ForwardTo)
		}
	}
	udpg.periodic()
	return udpg
}

func (udpg *UDPGate) periodic() {
	FreeIdleSockets(udpg)
	time.AfterFunc(60 * time.Second, udpg.periodic)
}


// http debug/ui

func (udpg *UDPGate) InitMux(mux *http.ServeMux) {
	mux.HandleFunc("/dmesh/udpa", udpg.HttpUDPNat)
	mux.HandleFunc("/dmesh/udp", udpg.HttpAllUDP)
}

func (udpg *UDPGate) HttpAllUDP(w http.ResponseWriter, r *http.Request) {
	udpg.udpLock.RLock()
	defer udpg.udpLock.RUnlock()
	json.NewEncoder(w).Encode(udpg.AllUdpCon)
}

func (udpg *UDPGate) HttpUDPNat(w http.ResponseWriter, r *http.Request) {
	udpg.udpLock.RLock()
	defer udpg.udpLock.RUnlock()
	json.NewEncoder(w).Encode(udpg.ActiveUdp)
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
// This is the loop reading from remote and forwarding to 'local', equivalent with
// the dialed connection in TCP proxy.
//
// localAddr is the accept remote address - received packets will be sent there.
// upstreamConn is the 'accepting connection' (for forward), or fallback for capture if a src-preserving
//   method doesn't exist. TUN can preserve src, so app can see the real remote add/port. Otherwise
//   the client will see the local address of this node, and the same port that is used with the remote.
// udpN is the 'dialed connection' -
func remoteConnectionReadLoop(gw *UDPGate, localAddr *net.UDPAddr, acceptConn *net.UDPConn, udpN *UdpNat, writer ugate.UdpWriter) {
	if DumpUdp {
		log.Println("Starting remote loop for ", localAddr, udpN.ReverseSrcAddr, udpN.DestAddr, udpN.Dest)
	}
	buffer := bufferPoolUdp.Get().([]byte)
	defer bufferPoolUdp.Put(buffer)

	for {
		// TODO: reuse, detect MTU. Need to account for netstack buffer ownership
		// upstreamConn is a UDP Listener bound to a random port, receiving messages
		// from the remote app (or any other app in case of STUN)
		size, srcAddr, err := udpN.UDP.ReadFromUDP(buffer[0:cap(buffer)])
		if err != nil {
			log.Println("UDP: close read loop for ", localAddr, err)
			return
		}

		udpN.LastRemoteIP = srcAddr.IP
		udpN.LastsRemotePort = uint16(srcAddr.Port)

		// TODO: for android dmesh, we may need to take zone into account.
		if writer != nil {
			if DumpUdp {
				log.Println("UDP Reverse: ", srcAddr, "->", localAddr)
			}
			rsa := srcAddr
			if udpN.ReverseSrcAddr != nil {
				// For UDP, must match the original address used by client
				rsa = udpN.ReverseSrcAddr
			}
			n, err := writer.WriteTo(buffer[:size], localAddr, rsa)
			if DumpUdp {
				log.Println("UDP Res DPME: ", rsa, "->", localAddr, n, err)
			}
		} else {
			if DumpUdp {
				log.Println("UDP direct RES: ", srcAddr, "->", localAddr)
			}
			_, err := acceptConn.WriteToUDP(buffer[:size], localAddr)
			if err != nil {
				log.Println("UDP Err to remote", err)
			}
		}

		udpN.LastRead = time.Now()
		udpN.RcvdPackets++
		udpN.RcvdBytes += size
	}
}

func forwardReadLoop(gw *UDPGate, l *ugate.Listener, udpL *net.UDPConn) {
	remoteA, err := net.ResolveUDPAddr("udp", l.ForwardTo)
	if err != nil {
		log.Println("Invalid forward address ",l.ForwardTo, err)
		return
	}
	if DumpUdp {
		log.Println("Starting forward read loop for ", udpL.LocalAddr(), l.ForwardTo, remoteA)
	}
	buffer := bufferPoolUdp.Get().([]byte)
	defer bufferPoolUdp.Put(buffer)
	for {
		// TODO: reuse, detect MTU. Need to account for netstack buffer ownership
		// upstreamConn is a UDP Listener bound to a random port, receiving messages
		// from the remote app (or any other app in case of STUN)
		size, srcAddr, err := udpL.ReadFromUDP(buffer[0:cap(buffer)])
		if err != nil {
			log.Println("UDP: close read loop for ", udpL.LocalAddr(), err)
			udpL.Close()
			return
		}

		packetSourceString := srcAddr.String()

		gw.udpLock.RLock()
		udpN, found := gw.ActiveUdp[packetSourceString]
		gw.udpLock.RUnlock()

		if !found {
			// port 0
			// TODO: attempt to use the localPort first
			udpCon, err := net.ListenUDP("udp", &net.UDPAddr{
				Port: 0,
			})
			setReceiveBuffer(udpCon, bufferSize)

			if err != nil {
				log.Println("udp proxy failed to listen", err)
				//FreeIdleSockets(udpg)
				continue
			}

			udpN = &UdpNat{
				UDP: udpCon,
			}
			udpN.DestAddr = remoteA
			udpN.Open = time.Now()
			udpN.LocalPort = udpCon.LocalAddr().(*net.UDPAddr).Port

			gw.udpLock.Lock()
			gw.ActiveUdp[packetSourceString] = udpN
			gw.udpLock.Unlock()

			log.Println("UDP-FW open: client:", srcAddr, "nat:", udpCon.LocalAddr(), "fw:", remoteA)

			go remoteConnectionReadLoop(gw, srcAddr, udpL, udpN, nil)
		}

		udpN.LastWrite = time.Now()
		udpN.SentPackets++
		udpN.SentBytes += size

		udpN.LastRemoteIP = srcAddr.IP
		udpN.LastsRemotePort = uint16(srcAddr.Port)

		if DumpUdp {
			log.Println("UDP-FW: ", srcAddr, "->", remoteA)
		}

		udpN.UDP.WriteToUDP(buffer[:size], remoteA)
	}
}

//func (p *Gateway) handlerRemotePackets() {
//	for pa := range p.upstreamMessageChannel {
//		p.UDPWriter.WriteTo(pa.data, pa.src)
//	}
//}

func (udpg *UDPGate) Listener(lc *ugate.Listener) {
	log.Println("Adding UDP ", lc)
	// port 0
	// TODO: attempt to use the localPort first
	ua, _ := url.Parse(lc.Address)
	_, p, _ := net.SplitHostPort(ua.Host)
	pp, _ := strconv.Atoi(p)
	udpCon, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: pp,
	})
	setReceiveBuffer(udpCon, bufferSize)

	if err != nil {
		log.Println("udp proxy failed to listen", err)
		return
	}

	go forwardReadLoop(udpg, lc, udpCon)
}

// HandleUDP is processing a captured UDP packet. It can be captured by iptables TPROXY or
// netstack TUN.
// Will create a NAT, using a local port as source and translating back.
func (udpg *UDPGate) HandleUdp(dstAddr net.IP, dstPort uint16, localAddr net.IP, localPort uint16, data []byte) {
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

	if dstPort == 53 && udpg.cfg.DNS != nil {
		udpg.cfg.DNS.HandleUdp(dstAddr, dstPort, localAddr, localPort, data)
		return
	}

	src := &net.UDPAddr{Port: int(localPort), IP: localAddr}

	packetSourceString := src.String()
	udpg.udpLock.RLock()
	udpN, found := udpg.ActiveUdp[packetSourceString]
	udpg.udpLock.RUnlock()

	if !found {
		udpCon, err := net.ListenUDP("udp", &net.UDPAddr{
			Port: 0,
		})

		if err != nil {
			log.Println("udp proxy failed to listen", err)
			FreeIdleSockets(udpg)
			return
		}

		setReceiveBuffer(udpCon, bufferSize)

		udpN = &UdpNat{
			UDP: udpCon,
		}

		l := udpg.cfg.FindRoutePrefix(dstAddr, dstPort, "udp://")
		if l.ForwardTo != "" {
			udpN.DestAddr, err = net.ResolveUDPAddr("udp", l.ForwardTo)
			if err != nil {
				log.Println("Failed to resolve ", l.ForwardTo, err)
				return
			}
			udpN.ReverseSrcAddr = &net.UDPAddr{IP:dstAddr, Port: int(dstPort)}
		} else {
			// Original destination
			udpN.DestAddr = &net.UDPAddr{
				IP:   dstAddr,
				Port: int(dstPort),
			}
		}

		udpN.LocalPort = udpCon.LocalAddr().(*net.UDPAddr).Port
		udpg.udpLock.Lock()
		udpg.ActiveUdp[packetSourceString] = udpN
		udpg.udpLock.Unlock()

		w := udpg.TransparentUDPWriter
		if w == nil {
			w = TransparentUDPWriter
		}
		// all packets on the source port will be sent back to localAddr:localPort
		go remoteConnectionReadLoop(udpg, src, udpCon, udpN, w)
	}

	n, err := udpN.UDP.WriteTo(data, udpN.DestAddr)

	udpN.LastWrite = time.Now()
	udpN.SentPackets++
	udpN.SentBytes += len(data)
	udpN.Open = time.Now()


	if DumpUdp {
		if found {
			log.Println("UDP OFW ", src, "->", udpN.DestAddr, n)
		} else {
			log.Println("UDP open ", src, "->", udpN.UDP.LocalAddr(), "->", udpN.DestAddr, n, err)
		}
		log.Println("UDP open ", src, "->", udpN.UDP.LocalAddr(), "->", udpN.DestAddr, n)
	}
}

const bufferSize = (1 << 20) * 2

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

func (gw *UDPGate) Close() error {
	gw.udpLock.Lock()
	//gw.closed = true
	for _, conn := range gw.ActiveUdp {
		conn.UDP.Close()
	}
	gw.udpLock.Unlock()
	return nil
}
