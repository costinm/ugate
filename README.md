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

For exampple, local Iperf3 without splice: 11Gbps, with splice: 21Gbps, 
raw (no proxy) 28.

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
- 

