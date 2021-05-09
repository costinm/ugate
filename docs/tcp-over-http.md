# API and protocol

The main function of uGate is to securely proxy 'streams' with basic metadata.


TODO: Attempt to emulate IPFS p2p URL structure and API.

Must also support/be compatible with Istio BTS.


# Transports

## WebSocket

This is the more basic transport, over plain HTTP. The websocket provides a framed
transport - we can map H2 frames to websocket frames. Raw H2 protocol can be used directly.

## HTTP/2

If another proxy is in front or a normal http stack is used -  most likely we can't use 'raw' H2,
For 'forward' streams - normal H2 streams can be used.
For 'reverse' - a POST request is made, and used as Raw H2.

## Raw H2

This is using the H2 framer - without going trough the HTTP library and semantics.

In particular: 
- FIN/RST can be sent in either direction. 
- no extra HTTP semantics processing
- Like QUIC, streams can be opened by either end.
- Like QUCI, stream types and headers are flexible, don't depend on HPACK and semantics


# Other attempts

Original protocol:


``` 
// URL format:
// - /tcp/HOSTNAME:PORT - egress using hostname (may also be ipv4 IP)
// - /tcp/[IPADDR]:PORT - egress using IP6 address
// - /tcp/[fd00::MESHID]:port - mesh routing. Send to the port on the identified node, possibly via VPN and gateways.
//
// - WIP: /tcp/[fd00::MESHID]/.../HOSTNAME:port - mesh circuit routing. Last component is the final destination.
//
// Returns 50x errors or 200 followed by a body starting with 'OK' (to flush headers)
// Additional metadata using headers.
```

Dial used the original request Body to create the outbound request, avoiding a pipe
but making the abstraction more complicated.


## 
