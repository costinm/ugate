package local

import (
	"expvar"
	"net"
	"sync"
	"time"

	"github.com/costinm/meshauth"
)

var (
	MetricActiveNetworks  = expvar.NewInt("net_active_interfaces")
	MetricChangedNetworks = expvar.NewInt("net_changed_interfaces_total")
	MetricLLReceived      = expvar.NewInt("ll_receive_total")
	MetricLLReceivedAck   = expvar.NewInt("ll_receive_ack_total")
	MetricLLReceiveErr    = expvar.NewInt("ll_receive_err_total")
	MetricLLTotal         = expvar.NewInt("ll_peers")
)

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

// link local announcements,discovery and messaging
type LLDiscovery struct {
	m sync.RWMutex
	Nodes map[string]*Node

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
	mcPort int

	// Additional UDP port.
	udpPort int

	// Defaults to 6970
	baseListenPort int

	// Listening on * for signed messages
	// Source for sent messages and multicasts
	UDPMsgConn *net.UDPConn

	// My credentials
	auth *meshauth.MeshAuth
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
