# WebRTC protocol support

The Device Mesh includes endpoints behind NAT or with
ad-hoc network connectivity.

TCP connection to a reachable destination are great for
long-lived connections, since NATs idle timeout is relatively
high ( 15 min... 1 h). However communication between 2 devices
behind NAT requires an expensive relay.

UDP allows peer to peer communication - but has a short idle
timeout.

Among UDP protocols, WebRTC is special because it also allows
browsers to directly participate and communicate in the mesh.


# Signaling / Discovery / Control Plane

WebRTC and peer to peer communication depends on a separate 'control plane' to exchange:

For Dial:
- address of a peer, real or TURN gateways. ( Endpoint in Istio terms )
- SHA of cert for DTLS for the peer.
- a password for the TURN server
- local public address (real, TURN or NAT), to send to the remote peer.

For Accept:
- notification - can be webpush or use a long-lived connection with the CP
- a descriptor for the connection (SDP), including client cert SHA, addresses
- TURN passwords if a TURN server is used.

The control plane does not need to be trusted.

## Connected signaling

The most battery and network efficient signaling for accept on a low-use server is using Webpush or GCM. 

An active server (in foreground and having active clients) should create one or more WebRTC association with 
its Gateway. The gateway should have a directly reachable address, so it doesn't need ICE. 
To avoid extra dependencies, the address of the gateway can be used as TURN server.

Alternatively, if the CP only support HTTPS and possibly only H1:
- use STUN to get candidate IPs
- make a single /register or /dial request, using self-signed JWT/VAPID 
- for accept, the request will be made periodically, to keep the port open
- for dial, the response will include a TURN server for destination. 

Server may use a database or DHT or K8S to track endpoints. 

## Disconnected signaling

Assuming all nodes are isolated and possibly on separate networks created using P2P:

- in each network nodes use mDNS or dmesh multicast to locate gateways. For P2P, the well-known
  address of the P2P server can be used.
- each gateway node operates a TURN server, capable of connecting to known nodes.
- each gateway node handles discovery

Like in dmesh, powered nodes will attempt to start P2P APs and act as gateways. If no gateway
is found un-powered nodes will take turn as gateways and AP.

For small meshes the gateways can create full corss-connection and have the full discovery database.
For large meshhes - possibly use IPFS DHT. 

If any node is connected to unmetered internet it can provide connectivity, as a TURN server.

## Addressing

1. Each node has a persistent EC256 cert, used for WebRTC DTLS. The SHA will be exchanged in the SDP address.
2. Based on the cert SHA, an IPv6 address is also generated. 
3. (optional) The cert can also be used as IPFS address.


For Webpush, a second EC256 certificate is used, with 'audience' the public EC256 of the control plane.
Since webpush cert can't be shared in browser, 2 certs need to be used. On native apps the cert can be the same.


# WebRTC and raw RTCP

- Max 64k 'streams' per association
- flow control per association, not per stream
- streams have priority, reliable/unreliable properties, message based


## Protocols and comments

- MUX
  - Byte0:
  - 0,1,2,3: STUN
  - (16,19: ZRTP (DH for RTP))
  - 20..63: DTLS (23==app data)
  - 64..79: TURN channel, 0..0x5000
  - (127..129: RTP)
    
- Quic doesn't seem to be multiplexable on same port - it would need to 
  implement equivalent of STUN!


- DTLS: small changes to TLS for UDP encap, seq. number is explicit. UDP packet start with TLS packet record.
  No session ID, only SRC/DST IP/Port can be used as identifier.
  Quic does have session number.

  - Type, version (254, 253), Epoch (per cipher - can be session ID ?), Seq, length, data
  - No association ID.
  - Handshake has extra fields: message seq, fragment off, fragment len

- SCTP-over-DTLS (RFC8261) -
  - DTLS connection before SCTP
  - multiple assocs over DTLS. SCTP port numbers for mux ?

- TURN
  - extends STUN
  - auth compatible with SIP, HMAC of message with same pass.
  - username can be synthetic - ex. a date, or the sha of cert.
  - pass needs to be deterministic - used to compute HMAC-SHA1

- [RFC8832](https://tools.ietf.org/html/rfc8832) WebRTC Data Channel Establishment Protocol
  All messages in the stream will have same priority. Odd/even IDs.
  Uses websocket subprotocol registry. 
  Inside a SCTP stream:
  MessageType(8b), ChannelType(8b), Pri(16), RelParam(32),
  Label(16b + N), Protocol(16b + N)
  SCTP PID=50

- SCTP(RFC4690) - stream control transmission protocol. Originally over IP.
  WebRTC uses UDP + DTLS, with mTLS. Cert SHA exchanged out of band, multiple ways.
  It is possible to use it over UDP, with TLS and HTTP/2 or TCP on top - but one handshake per stream, and the
  H2 multiplexing seems less efficient than the native sctp.
  
  Each side can open streams, options for allowing loss and out of order, message boundary.

    - Address may change during session, multiple addresses !
    
    - VerificationTAG == association ID (32 bit on each side)
    - Header: src/dst port (16bit), 32bit Verification, 32bit checksum
    - Chunk Type(8), Chunk Flag(8), Chunk Len(16)
    - selective ack, heartbeat, cookie, 
    - Inside chunk, TLV with 16 bit T/L
    
  SCTP data: 16 bit stream ID, TSN 32b(per chunk seq), stream seq number 16, 32 bit 'app identification'.

- SCTP over UDP RFC6951: 
    - 'IP address MUST NOT be listed'
    - SCTP sends heartbeats
    - port 9899 (sctp-tunneling)
    - store UDP src port/IP for the association
    
    - 

- STUN
    - Addr 4, value: 2112A442
    - stun.l.google.com:19302
    - can multiplex on same port
    - RFC 7983        Multiplexing Scheme Updates for RFC 5764  September 2016
    - _stun._udp.example.com -> 3478 UDP/TCP, 5349 TLS,   
    

- RTC - for media, control using RTCP. Not used in this project, essential for WebRTC media.
  Packet: 12B header, 32bit timestamp, 16bit seq. 10... start.
  MUX by dst port, each session should use a different port.


# Flow

Client:
- Create PeerConnection, add DataChannel
- SetLocalDescription, wait for iceGatheringState to complete
- GetLocalDescription - should have all local IPs plus STUN/TURN
- send offer - including all my IP:ports

The offer can be used to connect - it now acts as a server.

'Dial' will use the RTC to get the IPs of the other side and trigger
the connection. 
- locate the dest and forward the offer
- SetLocalDescription, SetRemoteDescription, wait for iceGatherState
- send answer.

# Middle proxies

WebRTC uses full e2e encryption. In the absence of direct connectivity, middle
boxes can only rely on UDP-level NAT. In case of P2P networks, the STUN layer
or special code must add the P2P node as a candidate, and create a binding.

A (P2P-client) -> GW1 (P2P-AP+CL) -> GW2 (P2P-AP) -> ... -> GW9 (P2P-AP) -> B(P2P-client)

There are only 2 options provided by the spec and browser impl:
- STUN. A will get GW1 as the single candidate, using GW1 as signaling server.
GW1 creates a UDP listener on 10.49.0.1:portGW1


# Pion Implementation

## DTLS 

- uses udp

## UDP

- uses transport (buffers)
- emulate net.Conn, using the src IP:port as session

Raw connections can be forwarded !

## Transport

- uses logging
- buffers
- virtual net for testing

## STUN

- no dependencies

