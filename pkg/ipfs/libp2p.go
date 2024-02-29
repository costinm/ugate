//go:build !IPFSLITE
// +build !IPFSLITE

package ipfs

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/costinm/meshauth"
	"github.com/costinm/ugate"
	"github.com/libp2p/go-libp2p/core/network"
	"log"
	"net"

	"github.com/ipfs/boxo/routing/http/client"
	"github.com/ipfs/boxo/routing/http/contentrouter"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr"
)

// ConnectionGater, Server
type IPFS struct {
	Host host.Host
}

func (ipfs *IPFS) FindPeer(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
	return peer.AddrInfo{ID: id}, nil
}

func (ipfs *IPFS) DialContext(ctx context.Context, net string, addr string) (net.Conn, error) {
	return nil, nil
}

func Init(ug *ugate.UGate) {
	ug.ListenerProto["ipfs"] = func(gate *ugate.UGate, l *meshauth.PortListener) error {
		InitIPFS(ug.Auth, l.GetPort())
		return nil
	}
}

// InitIPFS creates LibP2P compatible transport.
// Identity is based on the EC256 workload identity in auth.
//
// Routing is based on HTTP.
//
// Main purpose of this integration is to take advantage of public
// auto-relay code and infra, for control/signaling channels.
//
func InitIPFS(auth *meshauth.MeshAuth, p2pport int32) *IPFS {
	p2p := &IPFS{

	}

	sk, _, _ := crypto.ECDSAKeyPairFromKey(auth.Cert.PrivateKey.(*ecdsa.PrivateKey))

	la := []multiaddr.Multiaddr{}
	listen, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", p2pport))
	la = append(la, listen)

	finalOpts := []libp2p.Option{
		libp2p.Identity(sk),
		libp2p.ListenAddrs(la...),
		libp2p.ConnectionGater(p2p),
	}

	// Use a HTTP gateway for resolution - https://ipfs.io or https://cloudflare-ipfs.com/
	// Can also use mdns locally
	finalOpts = append(finalOpts,
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {

			resClient, err := client.New("http://127.0.0.1:11015")
			cr := contentrouter.NewContentRoutingClient(resClient)
			return cr, err
		}))


	h, err := libp2p.New(finalOpts...)
	if err != nil {
		panic(err)
	}

	p2p.Host = h

	// Add our stream handlers
	h.SetStreamHandler(Protocol, streamHandler)

	// Use the host 'event bus' to subscribe to all events.
	InitEvent(p2p.Host)

	for _, a := range h.Addrs() {
		log.Println("IPFS Addr: ", a.String() + "/p2p/" + h.ID().String())
	}
	log.Println("IPFS CID: ", peer.ToCid(h.ID()).String())

	return p2p
}


const Protocol = "/ugate/0.0.1"

func streamHandler(stream network.Stream) {
	// Remember to close the stream when we are done.
	defer stream.Close()

	log.Println("NEW STREAM: ", stream.Conn().RemotePeer(), stream.Conn().RemotePublicKey())

}
