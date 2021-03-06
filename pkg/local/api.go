package local

import (
	"net"
	"sync"

	auth2 "github.com/costinm/ugate/pkg/auth"
	ug "github.com/costinm/ugate/pkg/ugatesvc"
)

// link local announcements,discovery and messaging
type LLDiscovery struct {

	// Will be updated with the list of active interfaces
	// by Refresh() calls or provided by Android.
	// Key is the string representation of the address.
	ActiveInterfaces map[string]*ActiveInterface

	activeMutex sync.RWMutex

	// If set, the p2p- interface name of the current active AP
	// The name may change - used to adjust the address/zone of nodes.
	// All nodes have LL addresses including the old zone - might be better to remove any known node,
	// and wait for it to reannounce or respond to our announce. It'll take some time to reconnect as well.
	ActiveP2P string

	// Information about AP extracted from messages sent by the dmesh-l2 or Android application.


	// SSID and password of the AP
	AP     string
	APFreq string
	PSK    string
	// Set to the SSID of the main connection. 'w' param in the net status message.
	ConnectedWifi string
	WifiFreq      string
	WifiLevel     string

	// Port used to listen for multicast messages.
	// Default 5227.
	mcPort  int

	// Additional UDP port.
	udpPort int

	// Defaults to 6970
	baseListenPort int

	gw             *ug.UGate

	// Listening on * for signed messages
	// Source for sent messages and multicasts
	UDPMsgConn *net.UDPConn

	// My credentials
	auth *auth2.Auth
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
