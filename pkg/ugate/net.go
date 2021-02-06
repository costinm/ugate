package ugate

import (
	"time"

	"github.com/costinm/ugate"
)

// Keep track of network info - known nodes, active connections, routing.

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
	DialMUX(addr string, pub []byte, subs []string) (ugate.MuxedConn, error)
}



func NewDMNode() *ugate.DMNode {
	now := time.Now()
	return &ugate.DMNode{
		Labels:       map[string]string{},
		FirstSeen:    now,
		LastSeen:     now,
		NodeAnnounce: &ugate.NodeAnnounce{},
	}
}

