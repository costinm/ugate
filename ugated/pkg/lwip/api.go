package lwip

import (
	"io"
	"net"

	"github.com/songgao/water"
)

type TunConfig struct {
	Name string

	Sniff      bool

	UDPHandler UDPHandler
	TCPHandler TUNHandler
}

// UdpWriter is the interface implemented by the TunTransport, to send
// packets back to the virtual interface
type UdpWriter interface {
	WriteTo(data []byte, dstAddr *net.UDPAddr, srcAddr *net.UDPAddr) (int, error)
}

// Interface implemented by TUNHandler.
type UDPHandler interface {
	HandleUdp(dstAddr net.IP, dstPort uint16,
			localAddr net.IP, localPort uint16,
			data []byte)
}


// Interface implemented by TUNHandler.
// Important: for android the system makes sure tun is the default route, but
// packets from the VPN app are excluded.
//
// On Linux we need a similar setup. This still requires iptables to mark
// packets from istio-proxy, and use 2 routing tables.
//
type TUNHandler interface {
	HandleTUN(conn net.Conn, target *net.TCPAddr, la *net.TCPAddr) error
}

type CloseWriter interface {
	CloseWrite() error
}


// If NET_CAP or owner, open the tun.
func OpenTun(ifn string) (io.ReadWriteCloser, error) {
	config := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Persist: true,
		},
	}
	config.Name = ifn
	ifce, err := water.New(config)

	if err != nil {
		return nil, err
	}
	return ifce.ReadWriteCloser, nil
}


