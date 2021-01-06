package ugate

import (
	"encoding/json"
	"net"
	"time"
)

// Keep track of network info - known nodes, active connections, routing.

// A Connection that can multiplex.
// Will dial a stream, may also accept streams and dispatch them.
//
// For example SSHClient, SSHServer, Quic can support this.
type MuxedConn interface {
	// DialProxy will use the remote gateway to jump to
	// a different destination, indicated by stream.
	// On return, the stream ServerOut and ServerIn will be
	// populated, and connected to stream Dest.
	DialProxy(tp *Stream) error

	// The VIP of the remote host, after authentication.
	RemoteVIP() net.IP

	// Wait for the stream to finish.
	Wait() error
}

// Transport is creates multiplexed connections.
//
// On the server side, MuxedConn are created when a client connects.
type Transport interface {
	// Dial one TCP/mux connection to the IP:port.
	// The destination is a mesh node - port typically 5222, or 22 for 'regular' SSH serves.
	//
	// After handshake, an initial message is sent, including informations about the current node.
	//
	// The remote can be a trusted VPN, an untrusted AP/Gateway, a peer (link local or with public IP),
	// or a child. The subsriptions are used to indicate what messages will be forwarded to the server.
	// Typically VPN will receive all events, AP will receive subset of events related to topology while
	// child/peer only receive directed messages.
	DialMUX(addr string, pub []byte, subs []string) (MuxedConn, error)
}

// IPResolver uses DNS cache or lookups to return the name
// associated with an IP, for metrics/stats/logs
type IPResolver interface {
	IPResolve(ip string) string
}

// Information about a node.
// Sent periodically, signed by the origin - for example as a JWT, or UDP
// proto
type NodeAnnounce struct {
	UA string `json:"UA,omitempty"`

	// Non-link local IPs from all interfaces. Includes public internet addresses
	// and Wifi IP4 address. Used to determine if a node is directly connected.
	IPs []*net.UDPAddr `json:"IPs,omitempty"`

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

// Node information, based on registration info or discovery.
// Map of nodes, keyed by interface address is stored in Gateway.nodes.
type DMNode struct {
	// VIP is the mesh specific IP6 address. The 'network' identifies the master node, the
	// link part is the sha of the public key. This is a byte[16].
	// Last 8 bytes as uint64 are the primary key in the map.
	VIP net.IP `json:"vip,omitempty"`

	// Pub
	PublicKey []byte `json:"pub,omitempty"`

	// Information from the node - from an announce or message.
	NodeAnnounce *NodeAnnounce

	Labels map[string]string `json:"l,omitempty"`

	Bacokff time.Duration `json:"-"`

	// Last LL GW address used by the peer.
	// Public IP addresses are stored in Reg.IPs.
	// If set, returned as the first address in GWs, which is used to connect.
	// This is not sent in the registration - but extracted from the request
	// remote address.
	GW *net.UDPAddr `json:"gw,omitempty"`

	// Set if the gateway has an active incoming connection from this
	// node, with the node acting as client.
	// Streams will be forwarded to the node using special 'accept' mode.
	// This is similar with PUSH in H2.
	TunSrv MuxedConn `json:"-"`

	// Existing tun to the remote node, previously dialed.
	TunClient MuxedConn `json:"-"`

	// IP4 address of last announce
	Last4 *net.UDPAddr `json:"-"`

	// IP6 address of last announce
	Last6 *net.UDPAddr `json:"-"`

	FirstSeen time.Time

	// Last packet or registration from the peer.
	LastSeen time.Time `json:"t"`

	// In seconds since first seen, last 100
	Seen []int `json:"-"`

	// LastSeen in a multicast announce
	LastSeen4 time.Time

	// LastSeen in a multicast announce
	LastSeen6 time.Time `json:"-"`

	// Number of multicast received
	Announces int

	// Numbers of announces received from that node on the P2P interface
	AnnouncesOnP2P int

	// Numbers of announces received from that node on the P2P interface
	AnnouncesFromP2P int
}

func NewDMNode() *DMNode {
	now := time.Now()
	return &DMNode{
		Labels:       map[string]string{},
		FirstSeen:    now,
		LastSeen:     now,
		NodeAnnounce: &NodeAnnounce{},
	}
}

// Textual representation of the node registration data.
func (n *DMNode) String() string {
	b, _ := json.Marshal(n)
	return string(b)
}

// Return the list of gateways for the node, starting with the link local if any.
func (n *DMNode) GWs() []*net.UDPAddr {
	res := []*net.UDPAddr{}

	if n.GW != nil {
		res = append(res, n.GW)
	}
	if n.Last4 != nil {
		res = append(res, n.Last4)
	}
	if n.Last6 != nil {
		res = append(res, n.Last6)
	}
	return res
}

// Called when receiving a registration or regular valid message via a different gateway.
// - HandleRegistrationRequest - after validating the VIP
//
//
// For VPN, the srcPort is assigned by the NAT, can be anything
// For direct, the port will be 5228 or 5229
func (n *DMNode) UpdateGWDirect(addr net.IP, zone string, srcPort int, onRes bool) {
	n.LastSeen = time.Now()
	n.GW = &net.UDPAddr{IP: addr, Port: srcPort, Zone: zone}
}
func (n *DMNode) BackoffReset() {
	n.Bacokff = 0
}
func (n *DMNode) BackoffSleep() {
	if n.Bacokff == 0 {
		n.Bacokff = 5 * time.Second
	}
	time.Sleep(n.Bacokff)
	if n.Bacokff < 5*time.Minute {
		n.Bacokff = n.Bacokff * 2
	}
}

