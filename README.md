# ugate

Minimal TCP/UDP gateway, optimized for capturing outbound connections using 
few common protocols (iptables, SOCKS,
CONNECT), detecting traffic type and extracting metadata, and forwarding to a destination. 
TUN supported using LWIP in the costinm/tungate, used for android.

# Design

The uGate can be used in few roles:

1. Egress sidecar - captures outbound traffic, using SOCKS, iptables or TUN
2. Ingress sidecar - forwards 'mesh' traffic to local app
3. Ingress gateway - receives 'regular' traffic, forwards to the mesh
4. Egress gateway - forwards 'mesh' traffic to the internet.
5. Legacy egress - forwards mesh traffic to local non-mesh devices.
6. Message gate - can proxy WebPush messages, typically for control plane.

It is intended for small devices - android, OpenWRT-like routers, very small
containers, so dependencies are minimized. 

It is also optimized for battery operation - control plane 
interacts with the gate via encrypted Webpush or GCM messages. 

## Mesh protocols

Sidecars send and receive mesh traffic using HTTP/2 or WebRTC.
For HTTP/2, a custom SNI header is used. 

Gateways use SNI to route, using splice after the header is parsed.

Since uGate is optimized for devices which may be in home nets or in 
p2p ad-hoc networks, inside the mesh it will support WebRTC communication.
This is the main external dependency, used in the webrtc/ module.


## ReadFrom/splice

By avoiding wrappers and abstractions we can detect if both input and output are TcpConn and use the 'splice' call,
where the transfer between in and out is done in kernel space.

For example, current numbers on my server, using iperf3, in Gbps:

- direct/local (-c 5201): 29
- capture+proxy without splice: 11Gbps
- capture+proxy - splice: 21Gbps,
- capture + tlsOut + tlsIn + proxy (similar with Istio full path):
- capture + plainOut + plainIn + proxy (similar with Istio plain text):

This is useful for 'TURN'-style proxy, CONNECT and SNI, where the stream is already encrypted e2e and the gateway is not
adding an encryption layer. It doesn't help for encrypting, only on the middle boxes where encryption is not needed,
just routing.

Splice also helps when the app already encrypts, there is no need for a second encryption. This is handled by
auto-detecting TLS.

# TODO

- UDP
- P2: (separate repo) WebRTC/TURN/STUN compat - check perf against H2 and SNI routing
- P2: K8s compat  (konectivity ?), KNative
- P3: raw H2 implementation - just forward the frames, without decoding or re-encoding.
    - per stream flow control will be end-to-end.
    - extended PUSH support
- P1: Relay-over-SNI: register N plain text conns, with a signed/encrypted header. SNI will proxy connections without
  multiplex or encryption - using splice. Alternative to H2R

- P0: iptable redirect 80 and 443, HTTP proxy with subset of K8S HTTPRoute
- P3: Cert signing, using Istio and/or simplified API (json-grpc?)
- P0: mangled hostname: KEYID.namespace.TRUST_DOMAIN in certs and SNI routes. Use pod ID as SAN, Istio Spiffe based on
  SA from JWT + namespace
- P2: OIDC auth (to support certs), VAPID extensions for OIDC compat (send cert)
