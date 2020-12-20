# ugate

Minimal TCP/UDP gateway. 

Will listen to a port and accept connections, with an optional
header. Connections will be proxied to a dialed connection.

The dial details are abstracted - IPFS/LibP2P is one possible 
implementation. 

# Port mux 

This library uses cmux to allow a port to be shared and 
auto-detect protocols. 

# ReadFrom/splice

By avoiding wrappers and abstractions we can detect if both input and output are TcpConn and 
use the 'splice' call, where the transfer between in and out is done in kernel space.

Iperf3 without splice: 11Gbps, with splice: 21Gbps

This is useful for 'TURN'-style proxy, CONNECT and SNI, where the stream is already encrypted e2e
and the gateway is not adding an encryption layer. 
