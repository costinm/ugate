package ipfs

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"log"
	"net/http"
	"time"

	proto "github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipns"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/multiformats/go-multiaddr"

	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	mh "github.com/multiformats/go-multihash"
)

/*
DHT notes

- created as side effect of libp2p.New, via p2pRouting(Host) callback
- implements PeerRouting
- takes a datastore.NewMapDatastore() as param
-



*/

// ConnectionGater implementation
// WIP - will implement a policy to allow/deny based on RBAC

func (p2p *IPFS) InterceptPeerDial(p peer.ID) (allow bool) {
	//log.Println("IPFS: peerDial", p)
	peerDialCnt++
	return true
}

var dialCnt = 0
var peerDialCnt = 0

func (p2p *IPFS) InterceptAddrDial(id peer.ID, m multiaddr.Multiaddr) (allow bool) {
	dialCnt++
	//log.Println("IPFS: addrDial", id, m)
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
func (p2p *IPFS) InitEvent() {
	h := p2p.Host
	log.Println(h.EventBus().GetAllEventTypes())
	sub, err := h.EventBus().Subscribe(event.WildcardSubscription)
	if err != nil {
		log.Println(err)
	}
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
				log.Println("IPFS PeerProto ", v, v.Added, v.Removed)
			case event.EvtPeerIdentificationCompleted:
				//log.Println("IPFS Peer ", v)
			default:
				log.Printf("IPFS Event: %T %v\n", e, e)
			}
		}

	}()

	log.Println("IPFS WorkloadID: ", h.ID().String())
	log.Println("IPFS Addr: ", h.Addrs())
	log.Println("IPFS CID: ", peer.ToCid(h.ID()).String())

	//_, ch,routing.RegisterForQueryEvents(context.Background())
}

func P2PAddrFromString(c string) (*peer.AddrInfo, error) {
	ma, err := multiaddr.NewMultiaddr(c)
	if err != nil {
		fmt.Printf("Error %v", err)
		return nil, err
	}
	//"/ip4/149.28.196.14/tcp/4001/p2p/12D3KooWLePVbQbv3PqsDZt6obMcWa99YyqRWjeiCtStSydQ6zjH"
	pi, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		fmt.Printf("Error %v", err)
		return nil, err
	}
	return pi, nil
}

// StartListener a signed proto, key is the public key.
// 'IPNS' - typically the value is an ipfs address, but can be anything.
func (p2p *IPFS) Put(key ed25519.PrivateKey, v []byte, seq uint64, exp time.Duration) (string, error) {
	sk, _ := crypto.UnmarshalEd25519PrivateKey(key)

	rec, err := ipns.Create(sk, v, seq, time.Now().Add(exp))
	//ipns.EmbedPublicKey(sk.GetPublic(), rec)
	id, err := peer.IDFromPublicKey(sk.GetPublic())
	data, err := proto.Marshal(rec)

	// Store ipns entry at "/ipns/"+h(pubkey)
	err = p2p.DHT.PutValue(context.Background(), ipns.RecordKey(id), data)

	log.Println("Published ", id, err)

	// B58
	return id.String(), nil
}

func nsToCid(ns string) (cid.Cid, error) {
	h, err := mh.Sum([]byte(ns), mh.SHA2_256, -1)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(cid.Raw, h), nil
}

// Announce this node is a provider for the key.
// Normally used for IPFS, key is the sha of the content. Also
// used for advertise, in Discovery and autorelay.go (/libp2p/relay)
// According to routing.go, 24h is the normal validity, recommended 6hours refresh.
func (p2p *IPFS) Provide(ns string) error {
	// Alternative
	v, _ := nsToCid(ns)
	return p2p.Routing.Provide(context.Background(), v, true)
	//return p2p.DHT.WAN.Provide(context.Background(), v, true)
}

func (p2p *IPFS) Find(ns string) {
	v, _ := nsToCid(ns)
	rc := p2p.Routing.FindProvidersAsync(context.Background(), v, 10)

	for r := range rc {
		log.Println("Got ", r)
	}
}

// IPFS debug/control function
func (p2p *IPFS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	q := r.Form.Get("q")
	if q != "" {
		// Get the peer.WorkloadID
		pi, err := peer.Decode(q)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		if p2p.DHT != nil {
			ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
			pchan, err := p2p.DHT.GetClosestPeers(ctx, string(pi))
			for x := range pchan {
				log.Println(x)
			}

			// Sends FIND_NODE
			data, err := p2p.DHT.FindPeer(context.Background(), pi)

			// GetValue accepts /ipns/PEER and /pk/PEER
			//pkkey := routing.KeyForPublicKey(pi)
			//data, err := p2p.DHT.GetValue(context.Background(), pkkey, dht.Quorum(1))
			if err != nil {
				w.Write([]byte(err.Error()))
			} else {
				fmt.Fprintf(w, "%v", data)
			}
		} else {
			po := p2p.Host.Peerstore().Addrs(pi)
			log.Println(po)
		}
		return
	}
	h := p2p.Host

	c := r.Form.Get("c")
	if q != "" {
		//"/ip4/149.28.196.14/tcp/4001/p2p/12D3KooWLePVbQbv3PqsDZt6obMcWa99YyqRWjeiCtStSydQ6zjH"
		pi, err := P2PAddrFromString(c)
		if err != nil {
			return
		}
		err = h.Connect(context.Background(), *pi)
		if err != nil {
			fmt.Printf("Error %v", err)
			return
		}
		return
	}

	log.Println("Peers: ", h.Peerstore().Peers())
	for _, p := range h.Peerstore().Peers() {
		log.Println(h.Peerstore().PeerInfo(p))
	}

	log.Println("Conns: ", h.Network().Conns())
	p2p.DHT.Provide(context.Background(), peer.ToCid(p2p.Host.ID()), true)
}
