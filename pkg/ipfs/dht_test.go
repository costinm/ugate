
package ipfs

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/path"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/costinm/meshauth"
	"github.com/ipfs/boxo/routing/http/client"
	"github.com/ipfs/boxo/routing/http/contentrouter"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peerstore"

	"github.com/costinm/ugate/pkg/test"

	"github.com/multiformats/go-multiaddr"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/sec"
	"github.com/libp2p/go-libp2p/core/transport"

	blank "github.com/libp2p/go-libp2p/p2p/host/blank"
	"github.com/libp2p/go-libp2p/p2p/host/eventbus"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"

	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"

	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/libp2p/go-libp2p/p2p/net/upgrader"

	"github.com/libp2p/go-libp2p/p2p/transport/tcp"

	tls "github.com/libp2p/go-libp2p/p2p/security/tls"
)

/*

# s1

IPFS ID:  QmcGCepxB4ymJLkWjbaAMRUW6UXjW1KZAu8oBNh3DtfpNS
IPFS CID:  bafzbeigo37kn4faud7pazgd6pphlu64pifk6vti7gclc4jj4hdxjigphs4
[/ip6/2601:647:6100:449c:4ce5:460a:460:92ce/tcp/16100
/ip6/::1/tcp/16100
/ip4/10.1.10.228/tcp/16100
/ip4/127.0.0.1/tcp/16100]


# s2

2021/03/03 20:24:03 IPFS ID:  QmNZpRjHnEGkuHQezWHk4C7XGPfZxF1Kss3nV5JJMkwwVC
2021/03/03 20:24:03 IPFS Addr:  [/ip6/2601:647:6100:449c:4ce5:460a:460:92ce/tcp/15100 /ip6/::1/tcp/15100 /ip4/10.1.10.228/tcp/15100 /ip4/127.0.0.1/tcp/15100]
2021/03/03 20:24:03 IPFS CID:  bafzbeiadl6e43axbad4hegxivdsg6viv5hn6zopa77eonetyuc5bc4mplm



*/
const 	bobIPFSPeerID = "QmXpb6CSoNP6nMgq4kwY9Zd3Gysws1BzjXQd9cYN8CU3y9"
const mainPeerID = "QmNhttK3xm5LpyLVDBgzwhwyAsGkoMnYmEt1rmQvtMXg87"
const mainAddr = "/ip4/172.17.0.3/tcp/11014/p2p"

const testKey = "CAESQGsUl0stPGtM2PMq5rJArmHflcQfECHPY5p0V7vQSexKCJC7aR882u_lzQrvMeohGEvZTb39HiPzBiKPbDDww4w"

func TestPublicDHT(t *testing.T) {
	ctx := context.Background()
	kk, _ := base64.URLEncoding.DecodeString(testKey)
	kkk, _ := crypto.UnmarshalPrivateKey(kk)
	d := &IPFSDiscovery{Port: 8888, Bootstrap: dht.DefaultBootstrapPeers[0:2], Key: kkk}
	err := d.Init(ctx)
	if err != nil {
		t.Fatal(err)
	}

	//key, _ := crypto.MarshalPrivateKey(d.Key)
	//log.Println(base64.RawURLEncoding.EncodeToString(key))

	//pid, _ := peer.Decode(bobIPFSPeerID)
	//ai, err := d.DHT.FindPeer(context.Background(), pid)
	//log.Println(err, ai)

	l, _ := net.Listen("tcp", ":0")
	go http.Serve(l, d)

	p, _ := path.NewPath("/ipns/TEST")
	r, err := ipns.NewRecord(d.Key, p, 1, time.Now().Add(1 * time.Hour), 1 * time.Hour)//, ipns.WithPublicKey(true))
	rb, err := ipns.MarshalRecord(r)

	pkid, _ := peer.IDFromPublicKey(d.Key.GetPublic())
	name := ipns.NameFromPeer(pkid) // Names are in all cases based on the public key (or the public key)

	oldv, _ := d.DHT.GetValue(ctx, string(name.RoutingKey()))
	if oldv != nil {
		log.Println(oldv)
	}
	err = d.DHT.PutValue(ctx, string(name.RoutingKey()), rb)
	if err != nil {
		t.Fatal(err)
	}
	oldv, _ = d.DHT.GetValue(ctx, string(name.RoutingKey()))
	if oldv != nil {
		log.Println(oldv)
	}

	// URL: /routing/v1/peers/
	// -HAccept:application/x-ndjson,application/json
	// https://cloudflare-ipfs.com//routing/v1/peers/bafzbeiem4fnfiqwrhlt7scybi6bdnmommmtsdfwb5mcvsssqgl43xbiuba - 404
	// GET https://delegated-ipfs.dev/routing/v1/peers/bafzbeiem4fnfiqwrhlt7scybi6bdnmommmtsdfwb5mcvsssqgl43xbiuba - 500
	// cid.contact
	//resClient, err := client.New(
	//	"http://" + l.Addr().String())
	//	//"https://w3s.link")
	//	// "https://gw3.io") // text/html result
	////"https://dweb.link")
	//"https://cid.contact")
	//"https://gateway.ipfs.io")
	//"https://cloudflare-ipfs.com" )//
	//cr := contentrouter.NewContentRoutingClient(resClient)
	//cid := peer.ToCid(pid).String()
	//log.Println("CID", cid)
	//
	//pi, err := cr.FindPeer(context.Background(), pid)
	//log.Println(err, pi)
}

func TestLocalDHT(t *testing.T) {
	ctx := context.Background()

	root := &IPFSDiscovery{Port:  5555,  Domain: "mesh.internal"}
	err := root.Init(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rpi := peer.AddrInfo{ID: root.Host.ID(), Addrs: root.Host.Addrs()}

	nodes := []*IPFSDiscovery{}
	for i := 0; i < 3; i++ {
		n := &IPFSDiscovery{Port:  5600 + i, Domain: "mesh.internal"}// , Bootstrap: []multiaddr.Multiaddr{pi}}
		n.Init(ctx)
		//pi := peer.AddrInfo{ID: n.Host.ID(), Addrs: n.Host.Addrs()}
		// We ignore errors as some bootstrap peers may be down
		// and that is fine.
		n.Host.Connect(context.Background(), rpi)
		//root.Host.Connect(ctx, pi)
		nodes = append(nodes, n)
	}

	root.DHT.Bootstrap(ctx)

	// ipfs, ipld - CID
	// CID can contain 'identity', dns, multi-address, etc

	// TODO: for private DHT, add a Validator that allows any values, not only the public IPFS (ipns, etc)

	t.Run("ipns", func(t *testing.T) {
		// ipns - string
		p, err := path.NewPath("/ipns/bar")
		if err != nil {
			t.Fatal(err)
		}
		r, err := ipns.NewRecord(root.Key, p, 1, time.Now().Add(1 * time.Hour), 1 * time.Hour)//, ipns.WithPublicKey(true))
		rb, err := ipns.MarshalRecord(r)

		pkid, _ := peer.IDFromPublicKey(root.Key.GetPublic())
		name := ipns.NameFromPeer(pkid) // Names are in all cases based on the public key (or the public key)
		err = nodes[2].DHT.PutValue(ctx, string(name.RoutingKey()), rb)
		if err != nil {
			t.Fatal(err)
		}

		for _, nn := range nodes {
			v, err := nn.DHT.GetValue(ctx, string(name.RoutingKey()))
			if err != nil {
				t.Fatal(err)
			}
			r, err := ipns.UnmarshalRecord(v)
			if err != nil {
				t.Fatal(err)
			}
			path, err := r.Value()
			if err != nil {
				t.Fatal(err)
			}
			if "/ipns/bar" != path.String() {
				t.Fatal("invalid value", path)
			}
		}
	})

	t.Run("genvalue", func(t *testing.T) {
		log.Println("Put value")
		err = nodes[1].DHT.PutValue(ctx, "/test/bar", []byte("Hello"))
		if err != nil {
			t.Fatal(err)
		}
		log.Println("Get value")

		val, err := nodes[2].DHT.GetValue(ctx, "/test/bar")
		if err != nil {
			t.Fatal(err)
		}
		if string(val) != "Hello" {
			t.Error("got ", string(val))
		}
	})
	//log.Println(v)
}

func TestHttpRouting(t *testing.T) {
	pid, _ := peer.Decode(mainPeerID)


	// URL: /routing/v1/peers/
	// -HAccept:application/x-ndjson,application/json
	// https://cloudflare-ipfs.com//routing/v1/peers/bafzbeiem4fnfiqwrhlt7scybi6bdnmommmtsdfwb5mcvsssqgl43xbiuba - 404
	// GET https://delegated-ipfs.dev/routing/v1/peers/bafzbeiem4fnfiqwrhlt7scybi6bdnmommmtsdfwb5mcvsssqgl43xbiuba - 500
	// cid.contact
	resClient, err := client.New("http://localhost:11015")
	//"https://w3s.link")
	// "https://gw3.io") // text/html result
	//"https://dweb.link")
	//"https://cid.contact")
	//"https://gateway.ipfs.io")
	//"https://cloudflare-ipfs.com" )//
	cr := contentrouter.NewContentRoutingClient(resClient)
	//cid := peer.ToCid(pid).String()
	//log.Println("CID", cid)

	pi, err := cr.FindPeer(context.Background(), pid)
	log.Println(err, pi)
}

// getHostAddress returns the first address of the host, with peer ID added.
func getHostAddress(ha host.Host) []string {
	// Build host multiaddress
	hostAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", ha.ID()))

	res := []string{}
	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := ha.Addrs()
	for _, aa := range addr {
		res = append(res, aa.Encapsulate(hostAddr).String())
	}
	return res
}


func TestP2PHelpers(t *testing.T) {
	pa, err := P2PAddrFromString("/ip4/149.28.196.14/tcp/4001/p2p/12D3KooWLePVbQbv3PqsDZt6obMcWa99YyqRWjeiCtStSydQ6zjH")
	if err != nil {
		t.Fatal(err)
	}
	log.Println(pa.ID, pa.Addrs, pa.String())

}

func newHost(t *testing.T, listen multiaddr.Multiaddr) host.Host {
	h, err := libp2p.New(
		libp2p.ListenAddrs(listen),
	)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSimple(t *testing.T) {
	m1, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/10000")
	m2, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/10001")
	bob := newHost(t, m1)
	alice := newHost(t, m2)
	defer bob.Close()
	defer alice.Close()

	bob.Peerstore().AddAddrs(alice.ID(), alice.Addrs(), peerstore.PermanentAddrTTL)
	alice.Peerstore().AddAddrs(bob.ID(), bob.Addrs(), peerstore.PermanentAddrTTL)

	bob.SetStreamHandler("/test", func(stream network.Stream) {

	})
	
	ctx := context.Background()
	//alice.Connect(ctx, bob.ID())
	s1, err := alice.NewStream(ctx, bob.ID(), "/test")
	if err != nil {
		t.Fatal(err)
	}
	s1.Stat()
}

// Test a minimal config
func TestP2P(t *testing.T) {
	//a, _, _ := newHost( "5555", test.ALICE_KEYS)
	//b, _, _ := newHost( "5556", test.BOB_KEYS)
	a, _, _ := newBlankHost( "5555", test.AliceMeshAuthCfg)
	b, _, _ := newBlankHost( "5556", test.BobMeshAuthCfg)

	InitEvent(a)
	InitEvent(b)

	log.Println(a, b)


	ctx := context.Background()

	// To connect we need a peer address info
	// The transport created can Dial with address and peer ID (key)
	pi, err := P2PAddrFromString(getHostAddress(b)[0])
	log.Println("Dialing ", pi, "Addrs", b.Addrs())
	err = a.Connect(ctx, *pi)
	if err != nil {
		t.Fatal(err)
	}

	log.Println("Peers: ", a.Peerstore().Peers())
	for _, p := range a.Peerstore().Peers() {
		log.Println(a.Peerstore().PeerInfo(p))
	}
}


func newHostMin(port string, mc *meshauth.MeshCfg) (host.Host, multiaddr.Multiaddr, error)  {
	//// This is in the libp2p format (protobuf).
	ma := meshauth.New()

	//key, _ := os.ReadFile("testdata/s1/key")
	//kb, _ := base64.URLEncoding.DecodeString(string(key))
	//priv, _ := crypto.UnmarshalPrivateKey(kb)

	priv, _, _ := crypto.ECDSAKeyPairFromKey(ma.Cert.PrivateKey.(*ecdsa.PrivateKey))

	addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", port))
	if err != nil {
		return nil, nil, err
	}

	//peerID, err := peer.IDFromPrivateKey(priv)

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", port)),
		libp2p.Identity(priv),
		libp2p.DisableRelay(),
	}

	if false {
		opts = append(opts, libp2p.NoSecurity)
	}

	h, err := libp2p.New(opts...)
	return h, addr, err
}


func newBlankHost(port string, mc *meshauth.MeshCfg) (host.Host, multiaddr.Multiaddr, error) {
	ma := meshauth.New()

	//// This is in the libp2p format (protobuf).
	//key, _ := os.ReadFile("testdata/s1/key")
	//kb, _ := base64.URLEncoding.DecodeString(string(key))
	//priv, _ := crypto.UnmarshalPrivateKey(kb)

	priv, _, _ := crypto.ECDSAKeyPairFromKey(ma.Cert.PrivateKey.(*ecdsa.PrivateKey))

	tr, multiA, peerID, err := initTransport(port, priv)
	if err != nil {
		return nil, nil, err
	}

	// NewPeerstore creates an in-memory thread-safe collection of peers.
	// It's the caller's responsibility to call RemovePeer to ensure
	// that memory consumption of the peerstore doesn't grow unboundedly.
	ps, _ := pstoremem.NewPeerstore()

	// Must be added - host uses it to find its own key
	ps.AddPrivKey(peerID, priv)

	// BlankHost is used in testing - usually with a swarm for transport
	// "BlankHost is the thinest implementation of Host interface."
	// 2 options - WithConnManager and WithEventBus (default will be created)
	// Options can provide a non-null ConnMgr
	// In addition it has a 'mux', bus, network - currently Swarm is the only non-mock impl

	// ConnManager tracks connections to peers, and allows consumers to associate
	// metadata with each peer.
	//
	// It enables connections to be trimmed based on implementation-defined
	// heuristics. The ConnManager allows libp2p to enforce an upper bound on the
	// total number of open connections.
	//
	// ConnManagers supporting decaying tags implement Decayer. Use the
	// SupportsDecay function to safely cast an instance to Decayer, if supported.
	cm, err := connmgr.NewConnManager(1, 3)

	eb := eventbus.NewBus(eventbus.WithMetricsTracer(eventbus.NewMetricsTracer()))

	// The 'swarm' is responsible for handshake in user-space using the protocol WorkloadID.
	// No metadata.
	/*
	The IPFS Network package handles all of the peer-to-peer networking. It connects to other hosts, it encrypts communications, it muxes messages between the network's client services and target hosts. It has multiple subcomponents:

	- `Conn` - a connection to a single Peer
	  - `MultiConn` - a set of connections to a single Peer
	  - `SecureConn` - an encrypted (TLS-like) connection
	- `Swarm` - holds connections to Peers, multiplexes from/to each `MultiConn`
	- `Muxer` - multiplexes between `Services` and `Swarm`. Handles `Request/Reply`.
	  - `Service` - connects between an outside client service and Network.
	  - `Handler` - the client service part that handles requests

	*/
	n, _ := swarm.NewSwarm(peerID, ps, eb)
	n.AddTransport(tr)
	n.AddListenAddr(multiA)
	log.Println(n.ListenAddresses())

	h := blank.NewBlankHost(n, blank.WithEventBus(eb), blank.WithConnectionManager(cm))


	return h, multiA, err
}

// initTransport will return a configured transport for the port.
//
func initTransport(port string, priv crypto.PrivKey) (transport.Transport, multiaddr.Multiaddr, peer.ID, error) {
	addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", port))
	if err != nil {
		return nil, nil, "", err
	}

	peerID, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return nil, nil, "", err
	}

	// TODO: try webtransport
	// Don't bother with websocket

	// Set limits, track stuff
	rm := &network.NullResourceManager{}

	smu := []upgrader.StreamMuxer{{ID: yamux.ID, Muxer: yamux.DefaultTransport}}
	// Security - upgrades accepted and initiated conn
	//id := n.LocalPeer()
	//pk := n.Peerstore().PrivKey(id)
	//st := insecure.NewWithIdentity(insecure.ID, peerID, priv)
	st, _ := tls.New(tls.ID, priv, smu)

	u, err := upgrader.New([]sec.SecureTransport{st},
		smu, nil, nil,
		nil)

	t, err :=  tcp.NewTCPTransport(u, rm)

	ln, err := t.Listen(addr)
	if err != nil {
		return nil, nil, "", err
	}

	go func() {

		fmt.Printf("Listening. Now run: go run cmd/client/main.go %s %s\n", ln.Multiaddr(), peerID)
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Println("Listen stop", ln.Multiaddr(), err)
				return
			}
			log.Printf("Accepted new connection from %s (%s)\n", conn.RemotePeer(), conn.RemoteMultiaddr())
			log.Println(conn.RemotePeer().String(), conn.RemoteMultiaddr().String(),
				conn.RemotePublicKey())

			go func() {
				if err := handleConn(conn); err != nil {
					log.Printf("handling conn failed: %s", err.Error())
				}
			}()
		}
	}()

	return t, addr, peerID, nil
}

func handleConn(conn transport.CapableConn) error {
	str, err := conn.AcceptStream()
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(str)
	if err != nil {
		return err
	}
	log.Printf("Received: %s\n", data)
	if _, err := str.Write([]byte(data)); err != nil {
		return err
	}
	return str.Close()
}
