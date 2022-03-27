// +build !IPFSLITE

package ipfs

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/costinm/ugate/pkg/auth"
	"github.com/libp2p/go-libp2p/p2p/host/relay"
	ws "github.com/libp2p/go-ws-transport"
	"github.com/multiformats/go-multiaddr"

	"github.com/ipfs/go-datastore"
	config "github.com/ipfs/go-ipfs-config"
	ipfslog "github.com/ipfs/go-log"

	"github.com/ipfs/go-ipns"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	record "github.com/libp2p/go-libp2p-record"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	secio "github.com/libp2p/go-libp2p-secio"
	libp2ptls "github.com/libp2p/go-libp2p-tls"
)

type IPFSConfig struct {
	// If true enable the DHT functionality.
	// In this mode, we act as 'control plane'. If false, the node acts
	// as a gateway, using a CP for discovery and control.
	// The DHT requires maintaining a lot of connections, best for an internet
	// connected host - not for mobile.
	UseDHT bool



}

// ConnectionGater, Server
type IPFS struct {
	*IPFSConfig

	ctx context.Context
	Host host.Host

	// Only set for discovery nodes.
	// Mobile/battery powered/clients use Routing
	DHT  *dht.IpfsDHT

	// Combines ContentRouting, PeerRouting, ValueStore
	Routing routing.Routing // may be same as DHT
	store datastore.Batching
}

// DHT: supports putValue(key, value) for exactly 2 key types:
// - /pk/KEYHASH -> public key. Not relevant for ED keys ( key id is the key )
// - /ipns/KEYID -> proto IpnsEntry containing public key.



// InitIPFS creates LibP2P compatible transport.
// Identity is based on the EC256 workload identity in auth.
//
// Will use a select set of transports, remotely based on standards:
// - QUIC
// - WS
//
// UGate implements H2, WebRTC variants, with a IPFS transport adapter.
func InitIPFS(auth *auth.Auth, p2pport int, mux *http.ServeMux) *IPFS {
	p2p := &IPFS{
		ctx: context.Background(),
		IPFSConfig: &IPFSConfig{},
	}
	p2p.UseDHT = os.Getenv("DHT") == ""

	ctx := context.Background()
	//ipfslog.SetLogLevel("*", "debug")
	ipfslog.SetLogLevel("*", "warn")

	// Bootstrappers are using 1024 keys. See:
	// https://github.com/ipfs/infra/issues/378
	crypto.MinRsaKeyBits = 1024

	ds := datastore.NewMapDatastore()
	p2p.store = ds

	sk, _, _ := crypto.ECDSAKeyPairFromKey(auth.EC256Cert.PrivateKey.(*ecdsa.PrivateKey))
	// crypto.MarshalPrivateKey uses a protobuf - we use our own format.

	la := []multiaddr.Multiaddr{}
	listen, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", p2pport))
	la = append(la, listen)
	//listen, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", p2pport))
	//la = append(la, listen)
	//listen, err = multiaddr.NewMultiaddr(fmt.Sprintf("/ip6/::/tcp/%d", p2pport))
	//la = append(la, listen)
	listen, _ = multiaddr.NewMultiaddr(fmt.Sprintf("/ip6/::/udp/%d/quic", p2pport))
	la = append(la, listen)
	//listen, _ = multiaddr.NewMultiaddr("/ip4/0.0.0.0/udp/4005/quic")
	//la = append(la, listen)
	//listen, _ = multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/4006/ws")
	//la = append(la, listen)
	//listen, _ = multiaddr.NewMultiaddr("/ip6/::/tcp/4006/ws")
	//la = append(la, listen)

	// TODO: set a ConnectionGater !
	// TODO: equivalent StreamGater ?
	// TODO: create a ssh proxy
	// TODO: SSH transport

	//rtc := direct.NewTransport(
	//	webrtc.Configuration{},
	//	new(mplex.Transport),
	//)

	finalOpts := []libp2p.Option{
		libp2p.Identity(sk),
		libp2p.ListenAddrs(la...),
		libp2p.ChainOptions(
			libp2p.Transport(libp2pquic.NewTransport),
			libp2p.Transport(ws.New),
			//libp2p.Transport(rtc),
			//libp2p.DefaultTransports, // TCP, WS
		),

		libp2p.EnableNATService(),

		// If behind NAT, find and use relays
		libp2p.EnableAutoRelay(),
		// Accept from relay, initiate via relay. Required for auto relay
		libp2p.EnableRelay(), // no circuit.OptHop, OptActive

		// Attempt to use UpNP to open port
		// mostly uselss.
		//libp2p.NATPortMap(),

		libp2p.ConnectionManager(connmgr.NewConnManager(
			100,          // Lowwater
			600,          // HighWater,
			time.Minute, // GracePeriod
		)),

		// Used for the /ws/ transport - QUIC is 'capable', has own security
		// TODO: ssh over ws built in.
		// https://docs.libp2p.io/concepts/stream-multiplexing/#implementations
		// Defaults: mplex - no flow control
		// yamux - based on h2, but not the same. Problems closing. No JS.
		// spdystream - h2, has JS, based on docker/spdystream. Out of date, not core
		//
		//libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
		// Default: noise, tls
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		libp2p.Security(secio.ID, secio.New),

		libp2p.AddrsFactory(func(src []multiaddr.Multiaddr) []multiaddr.Multiaddr {
			res := []multiaddr.Multiaddr{}
			for _, s := range src {
				if strings.HasPrefix(s.String(), "/ip6/fd") {
					continue
				}
				if strings.HasPrefix(s.String(), "/ip4/10.") {
					continue
				}
				res = append(res, s)
			}
			return src
		}),
		libp2p.ConnectionGater(p2p),

		// Disables probing for rely, force using only public
		//libp2p.ForceReachabilityPublic(),
		//libp2p.ForceReachabilityPrivate(),

	}

	relay.DesiredRelays = 2

	if  p2p.UseDHT {
		//var ddht *dual.DHT
		finalOpts = append(finalOpts, libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			ddht, err := newDHT(ctx, h, ds)
			p2p.DHT = ddht
			//p2p.PeerStore = ddht
			p2p.Routing = ddht
			return ddht, err
		}))
	} else {
		// in memory peer store
		// HTTPS/XDS/etc for discovery, using a discovery server with DHT
		// or other backend.


		// This allows the node to be contacted behind NAT
		// Set the 'official' relays.
		// TODO: get relay dynamically, from discovery server
		finalOpts = append(finalOpts, libp2p.DefaultStaticRelays())

		// TODO: add option, use only if discovery server is local.
		finalOpts = append(finalOpts, libp2p.ForceReachabilityPrivate())
	}

	var pi *peer.AddrInfo
	// Connects to this node - mostly for testing.
	if rt := os.Getenv("IPFS_ROOT"); rt != "" {
		pi, err = P2PAddrFromString(rt)
		if err != nil {
			log.Println("Invalid ", rt, err)
		} else {
			//finalOpts = append(finalOpts, libp2p.StaticRelays([]peer.AddrInfo{*pi}))
		}
	}

	// Will register: (see multistream.AddHandlerWithFunc)
	// /ipfs/id -> handles peer registration in DHT
	// ping
	// secio
	// tls
	// yamux
	// mplex
	// libp2p/circuit/relay ( AddRelayTranport ) - if cfg.Relay
	// autonat -

	// On connect with a node, id is exchanged and the signed address record
	// is added to mem (addr_book.go ConsumePeerRecord).
	// By connecting to peers they learn the address, and may provide it.
	// FindPeer in DHT works by connecting to the actual peer, which must implement DHT
	// This is based on FIND_NODE messages.

	h, err := libp2p.New(
		ctx,
		finalOpts...,
	)
	p2p.Host = h
	log.Println("IPFS ID: ", h.ID().String())
	log.Println("IPFS Addr: ", h.Addrs())
	log.Println("IPFS CID: ", peer.ToCid(h.ID()).String())


	if err != nil {
		panic(err)
	}


	h.SetStreamHandler(Protocol, streamHandler)

	if pi == nil {
		cp, _ := config.DefaultBootstrapPeers()
		for _, addr := range cp {
			h.Connect(context.Background(), addr)
		}
	} else {
		// Maintain connections to all added peers.
		ps := NewPeeringService(h)
		ps.AddPeer(*pi)

		err = h.Connect(context.Background(), *pi)
		if err != nil {
			log.Println("IPFS: Failed to connect to ", *pi)
		} else {
			log.Println("IPFS: Connected to ", *pi)
		}
		ps.Start()

	}

	p2p.InitEvent()

	if p2p.DHT != nil && blockInit != nil {
		blockInit(p2p)
	}


	//p2p.DHT.Provide(context.Background(), peer.ToCid(h.ID()), true)
	return p2p
}

// Callback to initialize IPFS file handlers.
// Not needed for gateway, only if we want to serve some files.
var blockInit func(*IPFS)

// DHT results in a lot of connections. To debug, set a break on dial_sync (in swarm), in getActiveDial
// query.spawnQuery is causing the request
func newDHT(ctx context.Context, h host.Host, ds datastore.Batching) (*dht.IpfsDHT, error) {
	dhtOpts := []dht.Option{
		dht.NamespacedValidator("pk", record.PublicKeyValidator{}),
		dht.NamespacedValidator("ipns", ipns.Validator{KeyBook: h.Peerstore()}),
		dht.Concurrency(10),
		//dual.DHTOption(dht.Mode(dht.ModeClient)),
		//dual.DHTOption(dht.DisableAutoRefresh()),
		dht.QueryFilter(dht.PublicQueryFilter),
	}
	if ds != nil {
		dhtOpts = append(dhtOpts, dht.Datastore(ds))
	}

	return dht.New(ctx, h, dhtOpts...)
}

