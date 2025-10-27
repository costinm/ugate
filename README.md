# ugate

Originally, this was a minimal TCP/UDP gateway, optimized for capturing outbound connections using few common protocols (iptables, SOCKS,
CONNECT), detecting traffic type and extracting metadata, and forwarding to a destination. 

It first started as 'mesh for android' - with gvisor and LWIP to integrate with the stack.

It evolved as a 'modular monolith' integrating many protocols as a way to experiment and
adapt different components without the overhead of all boilerplate.

## Modules

This is using meshauth/ package for the core interfaces, auth, bootstraping and base config. 

The module is defined there - just a basic interface{} that may implement various go interfaces, as well as the most common config - Address, Destination, a HttpMux, etc.

Unlike other module abstractions (Caddy is a very good one), the actual module config is 
not embedded in the module definition. Each module is expecting to retrieve its configs,
based on the module name (and possibly the 'tenant' ID) from a config store. 

The config stores are also modules - meshauth/ defines file and env variables - with K8S or databases as other options. File is quite powerful by itself - adapters as rclone can be used along with the file adapter, it doesn't need to be a local file.


## Multi-tenant modular monolith

In a server like PostgresQL, you can run multiple databases for different users - with internal permissions and (some) isolation. Isolation is not perfect, so running separate
servers - on different containers/pods or nodes is needed in some cases.

There is an operational tradeoff that needs to be made by admin based on the needs of each user/tenant - including the budget/cost and expected usage. 

For uGate, the 'module' is a standalone component, implementing some Go interfaces, perhaps
few HTTP handlers - and having its own config.  Modules may find other modules by name or use HTTP in case the module is remote. 

The main requirement for tenancy is that each module allows multiple instances at the 
same time. Not all modules can do this - a HTTP listener module on port 8080 can't be 
multi-tenant, but it can dispatch to many 'routing' modules based on hostname. 

## CLI 

I believe a server should not have a CLI - it should be managed by a control plane or use a config store, with dynamic updates when possible. As such, the main() for the server 
does not depends on or use Cobra.

It is also useful to have real CLI to use on the client side - and Cobra seems the most common standard. It would be ideal to auto-generate the Cobra bindings from the 
config structs, but for now manual is fine. The 'ucli' package is also loading the 
module definitions and adds the cobra wrappers.

Some modules may be intended for CLI - but they should also be usable in the server,
with a HTTP API that creates the config. 

# Gateway Design

The uGate started as a mesh gateway for Android and VMs, so many modules are focused on 
this function.

For example:

1. Egress sidecar - captures outbound traffic, using SOCKS, iptables or TUN
2. Ingress sidecar - forwards 'mesh' traffic to local app
3. Ingress gateway - receives 'regular' traffic, forwards to the mesh
4. Egress gateway - forwards 'mesh' traffic to the internet.
5. Legacy egress - forwards mesh traffic to local non-mesh devices.
6. Message gate - can proxy WebPush messages, typically for control plane.

This is also optimized for battery operation - control planes should
interact with the gate via encrypted Webpush messages instead of expecting long-lived
connections. 

## Mesh protocols

While Istio uses HTTP/2 CONNECT, and uGate is attempting to support it for interop - the goal is to create a gateway between many protocols. 

The original protocol - which is still the 'main' one - is SSH, with a focus on interop and compatibility with standard SSH clients and servers, so any device having a ssh implementation can participate in the mesh. 

In addition, browsers were a target, so protocols like WebRTC data channels are supported.

Besides 'H2', 'SSH', 'WebRTC' - other protocols like IPFS/LibP2P, QUIC and others can be integrated.



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

- P1: register dialers ( webrtc, quic, etc) for muxed connections and streams
- P1: webrtc listener to create new peerconnection after one is used, dial to use a synth. SDP string.

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

