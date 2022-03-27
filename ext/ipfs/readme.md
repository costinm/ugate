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

