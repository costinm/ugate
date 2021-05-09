package lwip

import (
	"io"
	"log"
	"net"

	"github.com/eycorsican/go-tun2socks/core"
)

// Temp: should move to android or higher level package.

const (
	MTU = 1500
)

// LWIPTun adapts the LWIP interfaces - in particular UDPConn
type LWIPTun struct {
	lwip       core.LWIPStack
	tcpHandler TUNHandler
	udpHandler UDPHandler
}

// Called by udp_conn.newUDPConn. conn will hold a chan of packets.
// If err != nil - conn will be closed
// Else ReceiveTo will be called on each pending packet.
func (t *LWIPTun) Connect(conn core.UDPConn, target *net.UDPAddr) error {
	return nil
}

// Will get pending packets for 'connections'.
// The handling of udpRecvFn:
// - convert srcAddr/dstAddr params
// - srcAddr used to construct a 'connection'
// - if not found - construct one.
func (t *LWIPTun) ReceiveTo(conn core.UDPConn, data []byte, addr *net.UDPAddr) error {
	la := conn.LocalAddr()
	go t.udpHandler.HandleUdp(addr.IP, uint16(addr.Port), la.IP, uint16(la.Port), data)
	return nil
}

func (t *LWIPTun) Handle(conn net.Conn, target *net.TCPAddr) error {
	// Must return - TCP con will be moved to connected after return.
	// err will abort. While this is executing, will stay in connected
	// TODO: extra param to do all processing and do the proxy in background.
	go t.tcpHandler.HandleTUN(conn, target, nil)
	return nil
}

// Inject a packet into the UDP stack.
// dst us a local address, corresponding to an open local UDP port.
// TODO: find con from connect, close the conn periodically
func (t *LWIPTun) WriteTo(data []byte, dst *net.UDPAddr, src *net.UDPAddr) (int, error) {
	core.WriteTo(data, dst, src)
	return 0, nil
}

func NewTUNFD(tunDev io.ReadWriteCloser, handler TUNHandler, udpNat UDPHandler) *LWIPTun {

	lwip := core.NewLWIPStack()

	t := &LWIPTun{
		lwip: lwip,
		tcpHandler: handler,
		udpHandler: udpNat,
	}

	core.RegisterTCPConnHandler(t)
	//core.RegisterTCPConnHandler(redirect.NewTCPHandler("127.0.0.1:5201"))

	core.RegisterUDPConnHandler(t)
	core.RegisterRawUDPHandler(udpNat)
	
	core.RegisterOutputFn(func(data []byte) (int, error) {
		//log.Println("ip2tunW: ", len(data))
		return tunDev.Write(data)
	})

	// Copy packets from tun device to lwip stack, it's the main loop.
	go func() {
		ba := make([]byte, 10 *MTU)
		for  {
			n, err := tunDev.Read(ba)
			if err != nil {
				log.Println("Err tun", err)
				return
			}
			//log.Println("tun2ipR: ", n)
			_, err = lwip.Write(ba[0:n])
			if err != nil {
				log.Println("Err lwip", err)
				return
			}
		}
	}()

	return t
}
