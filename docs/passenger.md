# Passenger vs proxy sidecar

The term 'passenger' is based on ssh PROTOCOL.mux. I suspect many other words are used 
in the industry given the NIH traditions.

A pod-per-node, container-per-pod or process proxy will use a UDS to pass open
file descriptors, including memory mapped, to the sidecar. 

Wayland is a particular example - and makes the ideal protocol so we can reuse the
VM virtio work driven by ChromeOS. SSH is another broadly used protocol for passengers.

## mTLS SSH passenger

The SSH mux protocol defines a unix domain socket that will be used by ssh clients.
The server is typically a ssh in multiplex mode - but it can be anything, including
a ztunnel using mTLS and mesh protocols.

If the mux is set as a jump host - only one UDS is needed. 
