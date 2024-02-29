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

	"github.com/costinm/meshauth"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/peering"
	"github.com/ipfs/boxo/routing/http/server"
	"github.com/ipfs/boxo/routing/http/types"
	"github.com/ipfs/boxo/routing/http/types/iter"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"

	"github.com/libp2p/go-libp2p/core/host"
)

// Support a libp2p DHT proxy, using http API.
// Should run on a 'control plane' node - meshauth or some other infra node, ideally DNS and workload discovery server.
// Can also run standalone - to avoid every workload using libp2p having to interact with and be a DHT node.

// IPFSDiscovery registers in the libp2p DHT infra ( by default public - but can also be a private DHT mesh)
// and acts as a DHT node, implementing the http routing server API.
type IPFSDiscovery struct {
	DHT  *dht.IpfsDHT
	Host host.Host

	Port int
	Auth *meshauth.MeshAuth

	// gorilla mux with /routing path - not well integrated with http mux
	routingHandler http.Handler

	dialCnt     int
	peerDialCnt int
	securedCnt  int
	upgradedCnt int
	Bootstrap   []multiaddr.Multiaddr
	Key         crypto.PrivKey

	// If empty, this is used with the public infra
	Domain      string
}


// DHT: supports putValue(key, value) for exactly 2 key types:
// - /pk/KEYHASH -> public key. Not relevant for ED keys ( key id is the key )
// - /ipns/KEYID -> proto IpnsEntry containing public key.


var dialCnt = 0
var peerDialCnt = 0
var (
	securedCnt = 0
	upgradedCnt = 0
)

// NewDHT creates a DHT client for the public infra.
func NewDHT(ctx context.Context, auth *meshauth.MeshAuth, p2pport int) (*IPFSDiscovery, error) {
	i := &IPFSDiscovery{Auth: auth, Port: p2pport,
		Bootstrap: dht.DefaultBootstrapPeers,
		Domain : "",
	}
	err := i.Init(ctx)
	return i, err
}

type localValidator struct {

}

func (l localValidator) Validate(key string, value []byte) error {
	return nil
}

func (l localValidator) Select(key string, values [][]byte) (int, error) {
	return 0, nil
}

// NewDHT creates a host using DHT and router server.
func (ipfsd *IPFSDiscovery) Init(ctx context.Context) (error) {
	// crypto.MarshalPrivateKey uses a protobuf - we use our own format.

	//ipfslog.SetLogLevel("*", "debug")
	//ipfslog.SetLogLevel("*", "warn")

	// Bootstrappers are using 1024 keys. See:
	// https://github.com/ipfs/infra/issues/378
	crypto.MinRsaKeyBits = 1024

	connmgr, err := connmgr.NewConnManager(
		100, // Lowwater
		400, // HighWater,
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		return err
	}

	p2pport := ipfsd.Port
	la := []multiaddr.Multiaddr{}
	listen, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", p2pport))
	la = append(la, listen)

	listen, err = multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", p2pport + 1))
	la = append(la, listen)
	//listen, err = multiaddr.NewMultiaddr(fmt.Sprintf("/ip6/::/tcp/%d", p2pport))
	//la = append(la, listen)
	listen, err = multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d/ws", p2pport + 2))
	la = append(la, listen)

	//const webtransportHTTPEndpoint = "/.well-known/libp2p-webtransport"
	// for connect: add /certhash/HASH
	listen, err = multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic/webtransport", p2pport + 3 ))
	la = append(la, listen)


	//listen, _ = multiaddr.NewMultiaddr(fmt.Sprintf("/ip6/::/udp/%d/quic", p2pport))
	//la = append(la, listen)
	//listen, _ = multiaddr.NewMultiaddr("/ip4/0.0.0.0/udp/4005/quic")
	//la = append(la, listen)
	//listen, _ = multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/4006/ws")
	//la = append(la, listen)
	//listen, _ = multiaddr.NewMultiaddr("/ip6/::/tcp/4006/ws")
	//la = append(la, listen)
	//rtc := direct.NewTransport(
	//	webrtc.Configuration{},
	//	new(mplex.Transport),
	//)

	// TODO: set a ConnectionGater !
	// TODO: equivalent StreamGater ?
	// TODO: create a ssh proxy
	// TODO: SSH transport


	ds := datastore.NewMapDatastore()

	finalOpts := []libp2p.Option{
		libp2p.ListenAddrs(la...),
		//libp2p.ChainOptions(
		//	//libp2p.Transport(libp2pquic.NewTransport),
		//	//libp2p.Transport(ws.New),
		//	//libp2p.Transport(rtc),
		//	libp2p.DefaultTransports, // TCP, WS
		//),
		// support TLS connections
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		// support noise connections
		libp2p.Security(noise.ID, noise.New),
		// support any other default transports (TCP)
		libp2p.DefaultTransports,
		// Let's prevent our peer from having too many
		// connections by attaching a connection manager.
		libp2p.ConnectionManager(connmgr),
		// Attempt to open ports using uPNP for NATed hosts.
		libp2p.NATPortMap(),
		// If you want to help other peers to figure out if they are behind
		// NATs, you can launch the server-side of AutoNAT too (AutoRelay
		// already runs the client)
		//
		// This service is highly rate-limited and should not cause any
		// performance issues.
		libp2p.EnableNATService(),
		libp2p.ConnectionGater(ipfsd),

		// If behind NAT, find and use relays
		//libp2p.EnableAutoRelay(),
		// Accept from relay, initiate via relay. Required for auto relay
		//libp2p.EnableAutoRelayWithStaticRelays(relay), // no circuit.OptHop, OptActive

		// Attempt to use UpNP to open port
		// mostly uselss.
		//libp2p.NATPortMap(),

		// Used for the /ws/ transport - QUIC is 'capable', has own security
		// TODO: ssh over ws built in.
		// https://docs.libp2p.io/concepts/stream-multiplexing/#implementations
		// Defaults: mplex - no flow control
		// yamux - based on h2, but not the same. Problems closing. No JS.
		// spdystream - h2, has JS, based on docker/spdystream. RequestInPipe of date, not core
		//
		//libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
		// Default: noise, tls
		//libp2p.Security(secio.ID, secio.New),

		libp2p.AddrsFactory(func(src []multiaddr.Multiaddr) []multiaddr.Multiaddr {
			res := []multiaddr.Multiaddr{}
			for _, s := range src {
				if strings.HasPrefix(s.String(), "/ip6/fd") {
					continue
				}
				if strings.HasPrefix(s.String(), "/ip4/10.") {
					continue
				}
				if strings.HasPrefix(s.String(), "/ip4/127.") {
					continue
				}
				if strings.HasPrefix(s.String(), "/ip4/192.168") {
					continue
				}
				res = append(res, s)
			}
			return src
		}),
		// libp2p.ConnectionGater(p2p),

		// Disables probing for rely, force using only public
		//libp2p.ForceReachabilityPublic(),
		//libp2p.ForceReachabilityPrivate(),
	}
	finalOpts = append(finalOpts, libp2p.ForceReachabilityPrivate())

	if ipfsd.Key != nil {
		finalOpts = append(finalOpts, libp2p.Identity(ipfsd.Key))
	} else if ipfsd.Auth != nil && ipfsd.Auth.Cert != nil {
		sk, _, _ := crypto.ECDSAKeyPairFromKey(ipfsd.Auth.Cert.PrivateKey.(*ecdsa.PrivateKey))
		ipfsd.Key = sk
		finalOpts = append(finalOpts, libp2p.Identity(sk))
	} else {
		sk, _, _ := crypto.GenerateKeyPair(crypto.Ed25519, 2048)
		ipfsd.Key = sk
		finalOpts = append(finalOpts, libp2p.Identity(sk))
	}

	//relay.DesiredRelays = 2

	// This allows the node to be contacted behind NAT
	// Set the 'official' relays.
	// TODO: get relay dynamically, from discovery server
	// replaced by EnableAutoRelay
	//finalOpts = append(finalOpts, libp2p.DefaultStaticRelays())
	finalOpts = append(finalOpts,
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			dhtOpts := []dht.Option{
				//dht.NamespacedValidator("pk", record.PublicKeyValidator{}),
				//dht.NamespacedValidator("ipns", ipns.Validator{KeyBook: h.Peerstore()}),
				//dual.DHTOption(dht.Mode(dht.ModeClient)),
				//dual.DHTOption(dht.DisableAutoRefresh()),
				//dht.QueryFilter(dht.PublicQueryFilter),
			}
			if ipfsd.Domain == "" {
				// We may want to run an ipfs node to participate and provide a public server service, but not
				// with this library.
				dhtOpts = append(dhtOpts, dht.Mode(dht.ModeClient))
				//dhtOpts = append(dhtOpts, dht.Concurrency(3))
			} else {
				dhtOpts = append(dhtOpts, dht.Mode(dht.ModeServer))
				dhtOpts = append(dhtOpts, dht.Concurrency(3))
			}
			dhtOpts = append(dhtOpts, dht.Datastore(ds))
			// DHT results in a lot of connections. To debug, set a break on dial_sync (in swarm), in getActiveDial
			// query.spawnQuery is causing the request

			ddht, err := dht.New(ctx, h, dhtOpts...)
			if ipfsd.Domain != "" {
				ddht.Validator = &localValidator{}
			}
			ipfsd.DHT = ddht
			return ddht, err
		}))

	// Problem is that to expose this host we do need to push it to DHT.

	// in memory peer store
	// HTTPS/XDS/etc for discovery, using a discovery server with DHT
	// or other backend.

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

	// Connects to this node - mostly for testing.
	rt := os.Getenv("IPFS_ROOT")
	if rt == "" {
		rt = "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
	}

	h, err := libp2p.New(finalOpts...)
	if err != nil {
		panic(err)
	}

	ipfsd.Host = h
	
	

	// Use the host 'event bus' to subscribe to all events.
	InitEvent(h)

	ipfsd.StartDHT()

	ipfsd.routingHandler = server.Handler(ipfsd)

	// Maintain connections to all added peers. This generates a LOT of events
	if false {
		ipfsd.Peering(h, rt)
	}

	return nil
}

func (ipfsd *IPFSDiscovery) LogAddr()  {
	h := ipfsd.Host
	for _, a := range h.Addrs() {
		log.Println("IPFS Addr: ", a.String() + "/p2p/" + h.ID().String())
	}
	log.Println("IPFS CID: ", peer.ToCid(h.ID()).String())
}

func (d *IPFSDiscovery) StartDHT() {
	// This connects to public bootstrappers
	for _, addr := range d.Bootstrap {
		pi, _ := peer.AddrInfoFromP2pAddr(addr)
		// We ignore errors as some bootstrap peers may be down
		// and that is fine.
		d.Host.Connect(context.Background(), *pi)
	}
}

// Announce this node is a provider for the key.
// Normally used for IPFS, key is the sha of the content. Also
// used for advertise, in Discovery and autorelay.go (/libp2p/relay)
// According to routing.go, 24h is the normal validity, recommended 6hours refresh.
//func (p2p *IPFS) Provide(ns string) error {
//	v, _ := nsToCid(ns)
//	return p2p.Routing.Provide(context.Background(), v, true)
//}
//
//func (p2p *IPFS) Find(ns string) {
//	v, _ := nsToCid(ns)
//	rc := p2p.Routing.FindProvidersAsync(context.Background(), v, 10)
//
//	for r := range rc {
//		log.Println("Got ", r)
//	}
//}

// Additional ipfs features
func nsToCid(ns string) (cid.Cid, error) {
	h, err := multihash.Sum([]byte(ns), multihash.SHA2_256, -1)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(cid.Raw, h), nil
}


func (d *IPFSDiscovery) Peering(h host.Host, rt string) {
	pi, err := P2PAddrFromString(rt)
	if err != nil {
		log.Println("Invalid ", rt, err)
	} else {
		//finalOpts = append(finalOpts, libp2p.StaticRelays([]peer.AddrInfo{*pi}))
	}
	ps := peering.NewPeeringService(h)

	if pi != nil {
		ps.AddPeer(*pi)
		err = h.Connect(context.Background(), *pi)
		if err != nil {
			log.Println("IPFS: Failed to connect to ", *pi)
		} else {
			log.Println("IPFS: Connected to ", *pi)
		}
	}
	ps.Start()

}

// StartListener a signed proto, key is the public key.
// 'IPNS' - typically the value is an ipfs address, but can be anything.
//func (p2p *IPFS) Put(key ed25519.PrivateKey, v []byte, seq uint64, exp time.Duration) (string, error) {
	//sk, _ := crypto.UnmarshalEd25519PrivateKey(key)
	//
	//
	//
	//rec, err := ipns.Create(sk, v, seq,  time.Now().Add(exp), exp)
	////ipns.EmbedPublicKey(sk.GetPublic(), rec)
	//id, err := peer.IDFromPublicKey(sk.GetPublic())
	//data, err := proto.Marshal(rec)
	//
	//// Store ipns entry at "/ipns/"+h(pubkey)
	//err = p2p.DHT.PutValue(context.Background(), ipns.RecordKey(id), data)
	//
	//log.Println("Published ", id, err)
	//
	//// B58
	//return id.String(), nil
//	return "", nil
//}



// IPFS autoregistration and debug function.
//
// Normally should only be exposed to mesh workloads - i.e. the request should be over ambient or authenticated
// with JWT or mTLS.
//
//
func  (d *IPFSDiscovery) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/routing/") {
		d.routingHandler.ServeHTTP(w, r)
		return
	}
	r.ParseForm()

	q := r.Form.Get("q")
	if q != "" {
		// Get the peer.WorkloadID
		pi, err := peer.Decode(q)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
			ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
			pchan, err := d.DHT.GetClosestPeers(ctx, string(pi))
			for x := range pchan {
				log.Println(x)
			}

			// Sends FIND_NODE
			data, err := d.DHT.FindPeer(context.Background(), pi)

			// GetValue accepts /ipns/PEER and /pk/PEER
			//pkkey := routing.KeyForPublicKey(pi)
			//data, err := p2p.DHT.GetValue(context.Background(), pkkey, dht.Quorum(1))
			if err != nil {
				w.Write([]byte(err.Error()))
			} else {
				fmt.Fprintf(w, "%v", data)
			}
		return
	}

	h := d.Host

	c := r.Form.Get("c")
	if c != "" {
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

	c = r.Form.Get("p")
	if c != "" {
		// Provide indicates this node knows the value of the CID.
		d.DHT.Provide(context.Background(), peer.ToCid(d.Host.ID()), true)
		return
	}
}


func P2PAddrFromString(c string) (*peer.AddrInfo, error) {
	ma, err := multiaddr.NewMultiaddr(c)
	if err != nil {
		fmt.Printf("Error %v", err)
		return nil, err
	}
	//
	pi, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		fmt.Printf("Error %v", err)
		return nil, err
	}
	return pi, nil
}

// From kubo - but using DHT

type peerChanIter struct {
	ch     <-chan peer.AddrInfo
	cancel context.CancelFunc
	next   *peer.AddrInfo
}

func (it *peerChanIter) Next() bool {
	addr, ok := <-it.ch
	if ok {
		it.next = &addr
		return true
	}
	it.next = nil
	return false
}

func (it *peerChanIter) Val() types.Record {
	if it.next == nil {
		return nil
	}

	rec := &types.PeerRecord{
		Schema: types.SchemaPeer,
		ID:     &it.next.ID,
	}

	for _, addr := range it.next.Addrs {
		rec.Addrs = append(rec.Addrs, types.Multiaddr{Multiaddr: addr})
	}

	return rec
}

func (it *peerChanIter) Close() error {
	it.cancel()
	return nil
}

type contentRouter struct {
	DHT *dht.IpfsDHT
}

func (r *IPFSDiscovery) FindProviders(ctx context.Context, key cid.Cid, limit int) (iter.ResultIter[types.Record], error) {
	ctx, cancel := context.WithCancel(ctx)

	ch := r.DHT.FindProvidersAsync(ctx, key, limit)
	return iter.ToResultIter[types.Record](&peerChanIter{
		ch:     ch,
		cancel: cancel,
	}), nil
}

// nolint deprecated
func (r *IPFSDiscovery) ProvideBitswap(ctx context.Context, req *server.BitswapWriteProvideRequest) (time.Duration, error) {
	return 0, routing.ErrNotSupported
}

func (r *IPFSDiscovery) FindPeers(ctx context.Context, pid peer.ID, limit int) (iter.ResultIter[*types.PeerRecord], error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	addr, err := r.DHT.FindPeer(ctx, pid)
	if err != nil {
		return nil, err
	}

	rec := &types.PeerRecord{
		Schema: types.SchemaPeer,
		ID:     &addr.ID,
	}

	for _, addr := range addr.Addrs {
		rec.Addrs = append(rec.Addrs, types.Multiaddr{Multiaddr: addr})
	}

	return iter.ToResultIter[*types.PeerRecord](iter.FromSlice[*types.PeerRecord]([]*types.PeerRecord{rec})), nil
}

func (r *IPFSDiscovery) GetIPNS(ctx context.Context, name ipns.Name) (*ipns.Record, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	raw, err := r.DHT.GetValue(ctx, string(name.RoutingKey()))
	if err != nil {
		return nil, err
	}

	return ipns.UnmarshalRecord(raw)
}

func (r *IPFSDiscovery) PutIPNS(ctx context.Context, name ipns.Name, record *ipns.Record) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	raw, err := ipns.MarshalRecord(record)
	if err != nil {
		return err
	}

	// The caller guarantees that name matches the record. This is double checked
	// by the internals of PutValue.
	return r.DHT.PutValue(ctx, string(name.RoutingKey()), raw)
}

func (p2p *IPFSDiscovery) InterceptPeerDial(p peer.ID) (allow bool) {
	//log.Println("IPFS: peerDial", p)
	peerDialCnt++
	return true
}

func (p2p *IPFSDiscovery) InterceptAddrDial(id peer.ID, m multiaddr.Multiaddr) (allow bool) {
	dialCnt++
	if dialCnt <10 || dialCnt % 50 == 0 {
		log.Println("IPFS: addrDial", id, m)
	}
	return true
}

func (p2p *IPFSDiscovery) InterceptAccept(multiaddrs network.ConnMultiaddrs) (allow bool) {
	t, _ := multiaddrs.RemoteMultiaddr().MarshalText()
	t1, _ := multiaddrs.LocalMultiaddr().MarshalText()
	log.Println("IPFS: accept", string(t), string(t1))
	return true
}


func (p2p *IPFSDiscovery) InterceptSecured(direction network.Direction, id peer.ID, multiaddrs network.ConnMultiaddrs) (allow bool) {
	t, _ := multiaddrs.RemoteMultiaddr().MarshalText()
	securedCnt++
	if securedCnt <10 || securedCnt % 50 == 0 {
		log.Println("IPFS: secured", direction, id, string(t), dialCnt, peerDialCnt)
	}
	return true
}

func (p2p *IPFSDiscovery) InterceptUpgraded(conn network.Conn) (allow bool, reason control.DisconnectReason) {
	//t, _ := conn.RemoteMultiaddr().MarshalText()
	//rmt, _ := conn.RemoteMultiaddr().MarshalText()
	//upgradedCnt++
	//if upgradedCnt <10 || upgradedCnt % 50 ==0 {
	//	log.Println("IPFS: upgraded", conn.RemotePeer(), string(t),
	//		string(rmt), conn.ID())
	//}
	return true, 0
}
