# TL;DR

Experimental code to extract some ideas from IPFS/Libp2p - but no plan to use it, too many 
non-interoperable changes to protocols and complexity that is not needed.

# Important ideas

1. Pair a set of IP addresses with the SHA(public key)
2. Use SHA(public key) of each host (workload) as the identity to lookup IPs.
3. Support multiple protocols
4. built-in NAT/ICE

Not viable but interesting for corner use cases:
1. Global DHT for discovery (DNS replacement). In rare cases it may be useful, but a federated model is still decentralized but far more efficient/fast. 
2. Replacing DNS names with paths. 
3. Using unfriendly names (effectively UUIDs) for any human interaction.

Major flaws:
1. No access control ( there is some complicated way to have a separated mesh)
2. No separation between infra nodes (well connected, maintained) and small user nodes.
3. DHT treats everything as a reachable peer



# Intro

IPFS and libp2p are based on some (over-)complicated NIH protocols, but at the 
core some good ideas. While much simpler implementations would be desirable, IPFS 
also has a community and infrastructure that is useful

Similar (over complex, NIH) infra exists for Syncthing, Onion/Tor, etc - but
libp2p is more oriented to standalone use as a library and easier to run as 
a private network.

Example infra:

- cid.contact - route CID to host
  - alternative to using the DHT - too much bandwidth 
  - based on gossip pubsub instead of DHT

# Local DHT

The main problem with DHT is traffic and speed. This is in part due to the long-distance calls.

Libp2p DHT is tied to the protocols - and expects the peers to be connected. 

It looks like bittorent DHT is ligher and simpler. Not clear which has more nodes.


# HTTP routing 

https://specs.ipfs.tech/routing/http-routing-v1/ - URL  /routing/v1/providers/{cid}, returns 
a json with Addrs. Also /routing/v1/peers/{peer-id} - and ipns - returning signed record.

https://docs.ipfs.tech/how-to/address-ipfs-on-web/#path-gateway - various URL and DNS schemas

# Concepts

## Discovery

- DHT/kadelmia. Avoid 'dual' - it gets confused. LAN/local should 
  be kept separated, use auth.
- mDNS local
- also called 'signaling' in WebRTC 
- key is a SHA32 (of something), value is the address[]
- example key: sha("/libp2p/relay") - address are nodes providing relay service


## Circuits / Relay

- IPFS network provides a number of well-known and discoverable open relays
- useful for 'behind the NAT' - but should be used primarily for signaling
- IMO the use or relay for data is a major flaw in IPFS/LIBP2P. 

A better alternative is to use them only for push, with a larger set of 
nodes ( ideally all ), to initiate quic or webrtc connections. 
For fallback - standard TURN servers could be used.

This allows many home servers to get high speed ( no relay ).



## Record store

- Key is a public key - ideally ED25519 
- Value is a signed proto, typically a /ipfs address but it's a byte[]


# Transports

Unfortunately IPFS has major NIH problems around transports. While 
the concepts are great and similar to modern standards (H2, H3), they
made a choice to reject standards under the claim of 'decentralization'.

For example mTLS arbitrarily invents an new extension to pass public key, 
which makes absolutely no sense. Instead of 'decentralized' all protocols
are locked in to IPFS.

However it is possible to plug in different transports, and to use 
standard based ones instead. For communication with the current IPFS
infra - mainly DHT and circuit - we need to support at least TCP and 
their incompatible QUIC. 

For browsers - there is relatively little value in having browsers 
act as full DHT nodes. Browsers with WSS require servers with 
real certs - in which case it is better to use standard protocols
to the server, and have the server handle IPFS. 

## webrtc

- compatible with browsers, not well maintained (3/21 - doesn't compile)
- 'star' and 'direct'
- /dns4/wrtc-star1.par.dwebops.pub/tcp/443/wss/p2p-webrtc-star/p2p/<your-peer-id>
- star not supported in go, protocol not documented/clear
- incomplete, security not based on webrtc certs.



## ws + mplex + noise|secio

- primarily for use with browser-based nodes, without RTC
- requires DNS certs (apparently for wss)

## quic

- not compatible with browsers
- flow control on streams
- mostly standards based
- using modified TLS - so not interoperable with non-libp2p clients 

# Interfaces

## API

https://docs.ipfs.io/reference/http/api/

http://localhost:5001/api/v0/swarm/peers
query: arg (repeated), rest are flags.

Response: JSON


## Gateway

/p2p/${PEER_ID}/http/${PATH}

/p2p/${PEER_ID}/x/${PROTO}/http/${PATH}

