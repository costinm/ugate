module github.com/costinm/ugate/ext/ipfs

go 1.15

replace github.com/costinm/ugate => ../..

//replace github.com/libp2p/go-libp2p-webrtc-direct => ../../go-libp2p-webrtc-direct

require (
	github.com/costinm/ugate v0.0.0-20210221155556-10edd21fadbf
	github.com/gogo/protobuf v1.3.1
	github.com/ipfs/go-bitswap v0.3.3
	github.com/ipfs/go-blockservice v0.1.4
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-datastore v0.4.5
	github.com/ipfs/go-ipfs-blockstore v1.0.3
	github.com/ipfs/go-ipfs-chunker v0.0.5
	github.com/ipfs/go-ipfs-config v0.12.0
	github.com/ipfs/go-ipfs-provider v0.4.3
	github.com/ipfs/go-ipld-cbor v0.0.5
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-ipns v0.0.2
	github.com/ipfs/go-log v1.0.4
	github.com/ipfs/go-merkledag v0.3.2
	github.com/ipfs/go-unixfs v0.2.4
	github.com/karalabe/xgo v0.0.0-20191115072854-c5ccff8648a7 // indirect
	github.com/libp2p/go-libp2p v0.13.0
	github.com/libp2p/go-libp2p-blankhost v0.2.0
	github.com/libp2p/go-libp2p-connmgr v0.2.4
	github.com/libp2p/go-libp2p-core v0.8.0
	github.com/libp2p/go-libp2p-kad-dht v0.11.1
	github.com/libp2p/go-libp2p-mplex v0.4.1
	github.com/libp2p/go-libp2p-peerstore v0.2.6
	github.com/libp2p/go-libp2p-quic-transport v0.10.0
	github.com/libp2p/go-libp2p-record v0.1.3
	github.com/libp2p/go-libp2p-secio v0.2.2
	github.com/libp2p/go-libp2p-swarm v0.4.0
	github.com/libp2p/go-libp2p-tls v0.1.3
	github.com/libp2p/go-libp2p-webrtc-direct v0.0.0-20201219114432-56b02029fbb8
	github.com/libp2p/go-ws-transport v0.4.0
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/multiformats/go-multihash v0.0.14
	github.com/pion/webrtc/v2 v2.2.26

)
