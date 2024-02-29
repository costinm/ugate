# Monoliths, microservices and the middle ground

Skipping the rant part, it is important to find a middle
ground where users can take advantage of the benefits of
microservices with less costs. If the microservices
are all written in the same language and have about the
same security blast radius and RBAC permissions - combining
them in a monolith is not a bad idea, it is quite simple to
replace the remote calls to local calls and keep things
modular via reviews.

But this is not the only way, and it is not always the right way.

Modern server environments may have >200 cores (400+ vCPUs) and 10+ TB
or more RAM. In most prod environments you need at least 3 VMs
or k8s nodes for redundancy and multiple regions - but it is
perfectly possible to co-locate all micro-services on all
VMs. In K8S that would use DaemonSets.

This avoids sending the data over the wire - but it still requires
serialization and going trough the TCP stack.

The next step is to use pods with multiple containers and UDS
communication - it avoids the TCP stack, and UDS may send open
file descriptors which also allows passing shared memory areas.
But multiple containers on a pod impacts security boundaries.

We could use a CNI interface to inject UDS sockets into each
pod and allow cross-pod UDS communication - preserving security
boundaries.

A step further is injecting 'virtio' like interfaces allowing
shared memory and uring-style communication. UDS or similar
mechanisms can be used to detect and bootstrap this for containers,
and if strong isolation is desired real virtio between VMs on
same hosts can achieve similar 'zero copy'/shared memory communication.

The next problem is that shared memory still requires different
languages to marshall/unmarshall - or use the shared memory
natively for the buffers that need to be exchanged. Flatbuffers and
similar structures make this possible - but I think we should
look closer to WASM new structure definition as well.

While the new spec is intended for WASM to allow sharing data
between modules and host or between modules, and is a building
block for host-provided garbage collection - it is also a good
mechanism for shared memory communication between VMs on same
host or containers using shared memory.

# Ideal design (IMHO)

- uServices in same language and same security ( sharing a service
account, permissions, blast radius ) -> modular monolith
- different language, same security boundary -> containers on same pod, using
UDS or shared memory.
- different security boundary -> different Pods or VMs on same host,
using UDS/Virtio/shared memory

A single host should be able to hold the entire stack and
work fully independently - for the most part. Databases and
lock services would still need to be sharded and span multiple
hosts - plus some replication, but no wire traffic or TCP/TLS between
hosts should be needed except for the shared stateful data.

## Ambient

Istio is used to provide secure communication between apps, control
routing and provide basic telemetry. With ambient we are moving to
a per-node agent - ztunnel or built into CNI - which will encrypt
data between nodes but keep local traffic local.

In a world where uServices are optimized to avoid marshalling and wire
traffic, ztunnel can handle communication using TCP avoiding
some of the overheads - while still applying network policy and
some routing.

TODO: make sure 'node affinity' is correctly implemented...

What happens if apps start using UDS or shared memory for the
same-node communication - and how can we help ? Istio CNI can
start injecting UDS sockets into each pod ( in the abstract domain)
allowing communication with ztunnel or a similar per-node agent.

Ztunnel and CNI layer can act as a bus - applying policies and routing the calls, but
with a zero-copy semantics. Not clear if it can use a uRing interface or UDS
with shared file descriptros for memory or a combination, we need
to experiment with this.

Assuming the shared memory follows WASM layout, it will also allow WASM
'pods' and modules to participate with zero copy semantics.




