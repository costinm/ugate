package ipfs

import (
	"log"

	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/multiformats/go-multiaddr"
)


// ConnectionGater implementation
// WIP - will implement a policy to allow/deny based on RBAC

func (p2p *IPFS) InterceptPeerDial(p peer.ID) (allow bool) {
	log.Println("IPFS: peerDial", p)
	return true
}


func (p2p *IPFS) InterceptAddrDial(id peer.ID, m multiaddr.Multiaddr) (allow bool) {
	log.Println("IPFS: addrDial", id, m)
	return true
}

func (p2p *IPFS) InterceptAccept(multiaddrs network.ConnMultiaddrs) (allow bool) {
	t, _ := multiaddrs.RemoteMultiaddr().MarshalText()
	t1, _ := multiaddrs.LocalMultiaddr().MarshalText()
	log.Println("IPFS: accept", string(t), string(t1))
	return true
}


func (p2p *IPFS) InterceptSecured(direction network.Direction, id peer.ID, multiaddrs network.ConnMultiaddrs) (allow bool) {
	t, _ := multiaddrs.RemoteMultiaddr().MarshalText()
	log.Println("IPFS: secured", direction, id, string(t), dialCnt, peerDialCnt)
	return true
}

func (p2p *IPFS) InterceptUpgraded(conn network.Conn) (allow bool, reason control.DisconnectReason) {
	t, _ := conn.RemoteMultiaddr().MarshalText()
	rmt, _ := conn.RemoteMultiaddr().MarshalText()
	log.Println("IPFS: upgraded", conn.RemotePeer(), string(t),
			string(rmt), conn.ID())
	return true, 0
}

// Track events in the P2P implementation
func InitEvent(h host.Host) {

	//log.Println(h.EventBus().GetAllEventTypes())

	sub, err := h.EventBus().Subscribe(event.WildcardSubscription)
	if err != nil {
		log.Println(err)
	}

	connChgCnt :=0

	go func() {
		defer sub.Close()
		for e := range sub.Out() {
			switch v := e.(type) {
			case peer.ID:
				log.Println("IPFS Peer ", v)

			case event.EvtLocalAddressesUpdated:
				log.Println("IPFS local ", v)

			case event.EvtLocalReachabilityChanged:
				log.Println("IPFS reach ", v)

				case event.EvtPeerProtocolsUpdated:
					if len(v.Added) > 0 || len(v.Removed) > 0 {
						log.Println("IPFS PeerProto ", v, v.Added, v.Removed)
					}
			case event.EvtPeerIdentificationFailed:
				//log.Println("IPFS Peer ", v)
			case event.EvtPeerIdentificationCompleted:
				//log.Println("IPFS Peer ", v)
			case event.EvtPeerConnectednessChanged:
				connChgCnt++
				if connChgCnt % 50 == 0 {
					log.Println("PeerConnectednesChanged 50", v)
				}
			default:
				log.Printf("IPFS Event: %T %v\n", e, e)
			}
		}

	}()

	//_, ch,routing.RegisterForQueryEvents(context.Background())
}

