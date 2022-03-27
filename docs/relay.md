---
layout: post
title: Multinetwork via Relay
order: 10
layout: post
type: markdown
overview: Mesh multi-networking using existing relay protocols.
---

# Multi-networking using Relays



## Of-the-shelf relay software


### TURN 

- Each network runs a TURN server 
- Gateways or 'activators' in each network register to all other networks TURN servers
- The TURN address is saved in discovery server
- WebRTC used for stream tunneling. 
- Control plane, discovery and DNS interception used as 'signaling'

The relay - which is the most latency/scale critical component - will have a standard protocol with many implementations
designed to multimedia scale. 

Custom code will be the local tunnel ( iptables or local interception to WebRTC ) and the control plane for signaling.

### IPFS

- over complex
- the most interesting transports are H2 and H3 - but they are used in a non-standard, complicated way
- good concepts - but same can be achieved with simpler methods and using more standard implementation

### Syncthing

- https://docs.syncthing.net/specs/relay-v1.html
- existing infrastructure, also high volume.

### Tor


## SNI routing with mangled hostname

To preserve mTLS session, there are 3 main options:

- Relay uses SNI routing - the only available metadata is the hostname (port not included). Mangling can be used, 
for example _port._ip._network.svc... to connect to a svc in 'network' using network-local ip:port
The main problem is that there is n authentication for the client - it works well only if the relay is on a network
private address and is protected with NetworkPolicy. Main benefit is that it can work with unmodified native clients.

- Some prefix/handshake before the mTLS is exchanged
  - non-standard, doesn't work with a lot of infrastrucure. Using WS is an option that is generally supported.
  - can be spliced, fast performance after the handshake ( same as SNI routing )

- H2/H3 tunnel - like HBone/Masque. 


## H2R 

An old design was to reverse the H2 connection - the client would TCP connect, but the server would initiate the 
TLS and H2 handshakes as client. After this, the server could act as a relay for the client, using local discovery to
register itself on behalf of the client. 

This can work very well for H2, with a custom framer - most frames can be copied from one place to the other, but 
only if the relay terminates TLS. 

For tunneling raw mTLS connections - based on SNI or H2 metadata - the overhead seems too big, and a more basic
relay can take advantage of 'splice' and fast kernel paths.

