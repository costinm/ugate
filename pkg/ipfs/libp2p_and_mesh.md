# Background

# Simple usage

Because LibP2P and IPFS are quite heavy and complex, and use non-standard variation of protocols - they don't make 
a very good option as core protocol for sidecars or per-node proxies. 

Having Ingress and Egress gateways that support the core LibP2P protocols and bridge to mesh protocol (CONNECT or 
others) seem a much safer and simpler option in the data path. 

The Gateways can use the mesh control plane and discovery - including K8S - to locate other Gateways, without
the use of DHT. 

# Public DHT integration

A node that enables DHT becomes a full participant in the discovery and metadata storage protocol, which is
quite expensive. Resolving via DHT is also pretty slow - the nodes storing the key are remote. 

It can still be useful to have one (or few) servers - to reduce the use of IP addresses they may use
a couple of different ports in the gateway. The peer ID of the DHT nodes will be discoverable (slowly)
in DHT as a backup and for external clients. 

The big problem is that exposing services by internal nodes (gateways or workloads using libp2p) is not possible
with the normal protocol unless they are participatin in DHT. 

There is a simple solution: use IPNS to store records for each peer instead of peer ID. 

# Private DHT

# Using ideas from LibP2P in mesh

The core idea is tracking the public key (or SHA) along with the direct and relayed addresses. For regular 
mesh workloads it is expected that a common root CA is used - or Cluster configures trusted roots for multiple
hosts in a more efficient way. 

For individual hosts that are not part of a cluster - it is useful to save the public key hash or even the 
full key in the pod status. The private key can also be used with SSH and push protocols as well as for signing.



