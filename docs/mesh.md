# Meshes

Mesh may mean many things, and Istio has further confused the term by arbitrarily adding
observability, which is a general concept.

I believe 'mesh' is primarily about identity and security across multiple environments, 
and policies and telemetry based on identity and secure information, with the 
secure parts of the Internet and mobile as the largest public mesh.

In regular K8S or on most IPSec/Wireguard/proprietary networks, the policies are expressed
using authenticated IP address and associated trusted discovery server.

The policies are applied at the network level - by CNI or firewall - the only identity
they see is the source or dest IP. The low level keys provisioned on each node authenticate
the IP, so it can't be forged and MITM is not possible. As long as you are in the secure
network boundaries - and the IP is not re-allocated - you can trust the IP. 

Re-allocation and dynamic assignment of IPv4 has important risks and implications, can be 
discussed separately, but secure networks do support static or subnet allocations too 
that are more durable.

Inside the secured network - the associated discovery server holds metadata for the IP.
For K8S it's extensive - the full Pod spec, including a K8S specific 'account'. In other
networks it can be just a FQDN (hostname) and associated 'owner'. 

## Metadata and Discovery servers

Inside the boundaries of a secure network, we know the IP is authenticated and an ephemeral 
but secure identity. From an application perspective - it is the ONLY information they 
see with the standard POSIX/linux APIs. 

Even with Istio sidecars - the mTLS is hidden from the user and the client identity
only made available as an envoy-specific complex header (XFCC). In Istio ambient things
are worse for L4 - and with Waypoints it's not yet clear how identity will be passed.
To make things worse - Istio doesn't even propagate the real source IP with sidecars
unless TPROXY is used ( and it requires granting high permissions to the sidecar).

Many applications need to make decisions based on the caller identity - and usually
have their own separate authentication, sometimes using user/password or mTLS or JWTs.
Use of the source IP is rarely or never used besides logging - and the real source
is also hidden in Forwarded headers in HTTP and HA-Proxy prefix for TCP.

RFC912 and successors (from 1984) originally titled "Authentication service" defines
one of the first protocols to map TCP level  info available to apps back to 
identity. It takes src/dst ports and issues a request to the remote host. The 
remote host will see the source/dst IP on the callback, and 
use the extra src/dst to lookup the user identiy on its side - and return the 
identity on the host. Assuming the IP network is secure - it mostly works. 

Modern metadata servers run on a host-only address (like 169.254.169.254) and provide
tokens and trusted info from the discovery and auth systems. Like Ident, it needs to
find caller identity on the local host based on IP - however in most cases they only
use the IP (local call - either same network namespace or a local container). They 
keep track of all containers running locally - and use the discovery server to 
know identity and metadata. 



## Native mesh - or proxyless

A native mesh handles authentication, security and policy at the application level using
libraries or frameworks. This is what applications have been doing since the 
discovery of security - what makes it a 'mesh' is the presence of a discovery server
that primarily provides a mapping between 'mesh identity' and current IP, as well
as optional security options.


## User and host identities

From an application perspective, the pod or host service accounts are not as important
as a user identity ( or service account ) that owns the data. In many cases for 
container-based and micro-services the identity of container can be treated as 
the identity of the user - howerver even in this case there are multiple containers
running in multiple environments that hold the same identity.

Modern secure servers attempt to bind user to host - or apply different policies for
a user depending on the host, for example allow access to less data if user is using 
a device not owned/controlled by the org like a personal phone.









