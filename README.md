# ugate

Minimal TCP/UDP gateway, optimized for capturing outbound connections 
using few common protocols (iptables, SOCKS, CONNECT), detecting traffic
type and extracting metadata - and forwarding L4 traffic to a destination.

On the receive side it can optionally process a metadata header, including
extracting it from the SNI. 

Connections will be proxied to a dialed connection
The dial, the details are abstracted - IPFS/LibP2P is one possible 
implementation. It is also possible to use a custom listener.

# Port mux 

This library started by using cmux to allow a port to be shared and 
auto-detect protocols. In practice this is important for TLS and 
HTTP/1.1 vs HTTP2 and for auto-detecting HA-PROXY metadata. 

Instead the code has been optimized and simplified, with less generic
matching but specialized to the supported protocols.

# Midle boxes - routers

# ReadFrom/splice

By avoiding wrappers and abstractions we can detect if both input and output are TcpConn and 
use the 'splice' call, where the transfer between in and out is done in kernel space.

For example, current numbers on my server, using iperf3, in Gbps:
- direct/local (-c 5201): 29
- capture+proxy without splice: 11Gbps 
- capture+proxy - splice: 21Gbps, 
- capture + tlsOut + tlsIn + proxy (similar with Istio full path): 
- capture + plainOut + plainIn + proxy (similar with Istio plain text): 

This is useful for 'TURN'-style proxy, CONNECT and SNI, where the stream is already encrypted e2e
and the gateway is not adding an encryption layer. It doesn't help for 
encrypting, only on the middle boxes where encryption is not needed, just routing.

Splice also helps when the app already encrypts, there is no need for 
a second encryption. This is handled by auto-detecting TLS. 

# Wait-less sniffing

One idea to avoid the 'client first' problem in Istio is to open the 
connection to the dest immediately (if dest is known), and detect on 
the first bytes. This can work with socks/iptables outbound. 

# TODO

- UDP
- TURN/STUN compat
- K8s compat  (konectivity)
- TLS and metadata hooks
- H2 implementation can just forward the frames, without decoding or re-encoding. 
    - per stream flow control will be end-to-end.
- Relay-over-SNI: register N plain text conns, send signed header, SNI will proxy connections
without multiplex or push. 

