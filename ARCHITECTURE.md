# Core concepts

## Streams with metadata

The 'stream' is modeled after [H2 and H3](https://httpwg.org/specs/rfc7540.html#StreamsLayer). 
In go, this is modeled as a net.Conn interface - extended with metadata.

Like Istio, this library is based on 'enhanced' L4 streams, i.e. mostly opaque streams
with added identity, security and metadata. HTTP requests are also mapped to streams, 
so all communication is handled in an uniform way. uGate was designed around a messaging 
model - messages are also modeled as streams. 

## Messaging/pubsub

The original project ( wpgate ) goal was to implement a distributed/federated Webpush 
messaging infrastructure, to support a disconnected, ad-hoc communication (dmesh). 

Secure messaging and webpush remain at the core of uGate project.  

WIP. The 'control plane' portion is based on Istio XDS, but extended with the proposed 
'messaging' extensions.

Messages can also be transmitted as Webpush - i.e. encrypted, authenticated - so 
XDS proxies can't see or modify control messages.

## Associations, Circuits and reverse accept


Similar with SSH, Tor, IPFS, WebRTC, etc this allows nodes behind NAT and without
server ports to listen and accept streams and messages originated from other nodes.


# Code organization

## github.com/costinm/ugate

The top directory contains:

- core data structure like Stream, DMNode
- config structs
- interfaces implemented or used in the code.

The module has no external dependencies ( except golang.org/x/ ).

The ugate/pkg contains basic implementation - without external deps:

- auth - all authentication, certs, crypto, identity.
- cfgfs - a file/memory based implementation of the ConfigStore interface
- iptables - istio-compatible iptables capture, plus tproxy (WIP)
- local - multicast-based discovery of neighbors
- pipe - copied from private golang impl, used to turn http requests to net.Conn
- sni - SNI sniffing and routing ( like Istio )
- socks - socks capture
- udp - the UDP proxy and related code
- ugatesvc - the main service, including routing, listening and core proxy functions.
- webpush - implements Webpush protocol, used for messaging/control.

Integrations have separate go.mod files.

## github.com/costinm/quic

Depends on a forked lucasclemente QUIC implementation. Main change is to expose few
private methods, to allow processing individual streams. WIP: PR to upstream.

It will start a QUIC server and adapt QUIC streams to Stream interface, allowing
both ends to initiate streams. The base QUIC protocol already supports this - main
complexity is adapting this to the library's H3 implementation.

The Streams are mapped to H3 request responses, using non-standard reverse requests - 
the H3 standard does not allow server to initiate streams, despite QUIC supporting this.

## webrtc

WIP - integration with the 'github.com/pion' libraries. 

- allow browsers to participate in the mesh, using WebRTC data channels and associated DTLS certificates.
- STUN and TURN support - to enable peer-to-peer communication behind NATs

## XDS

WIP - Integration with Istio and other XDS servers. Will implement the ConfStore interface
and provide both XDS server and client interfaces, acting as a proxy.

The XDS protocol will also be used as alternate transport for Webpush messaging.

# External repos

- istio/api, k8s - to avoid large deps, the core interfaces/structs used are copied in the 
xds and auth directories. For example the credentials are stored in KubeConfig format.

- github.com/costinm/tungate - TUN support. Currently lwIP library is used, but it also 
include previous adapters using old 'netstack' and new 'gvisor' IP stacks. 

- github.com/costinm/dmesh - 'device mesh' contains Android implementation and adapter.
The uGate library is used as a JNI. Also uses the tungate to implement the Android VPN interface.
  
- pion - used for WebRTC, TUN, TURN

- lucasclemente - QUIC
