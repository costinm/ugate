# API and protocol

TODO: Attempt to emulate IPFS p2p URL structure and API.

Implementation compatible with Istio BTS.

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
