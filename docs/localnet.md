# Local host and the mesh

Containers and VMs work very hard to act as a separate, standalone machine - with their own network stack and IP address. 
Looking back, we started with 'large' servers shared by many users, batch processes and applications, moved to 
one and then many machines per user and large number of servers in cloud providers. But we also moved back to very
large servers with many cores and huge memory shared by many users. 

For a long time developers using bad abstractions and interfaces had a hard time understanding that a local call
is very different from a cross-region network call - latency, common errors, marshalling costs. Now we are at
a point where we need developers to understand that communication between 2 processes on the same machine is
very different from network calls. 

I was ranting about microservices and monoliths craziness - but I think the core of the confusion is the 
continuing attempts to abstract and hide the hardware architecture and differences between network calls
and communication between 2 processes on the same machine. 

K8S provides nice abstractions and mechanisms to schedule pods on specific node pools or nodes. Affinity,
taints, services with node preference, dedicated node pools can all be used to co-locate specific 
services, preserving the isolation and security boundaries but with 'same machine' instead of 'over the wire'
semantics.

## mTLS for interprocess communication

Rarely actually necessary - but workloads (proxyless) or sidecars (envoy, ztunnel) can use mTLS even when
communicating on the same machine. Will still be much faster than over the wire - but most optimizations
are lost. 

If you are considering combining 2 uServices with different security boundaries in a monolith - the
communication between them will clearly not be mTLS, and having them on the same machine isolated but
using a local IPC instead of mTLS over TCP is strictly better. 

TODO: expand

## Protocols

TCP is the easiest option, with minimal or no code changes when moving uServices to the same machine. 
For same machine you can avoid mTLS - but still need some ways to identify the peer. That's the job of 
an MDS that tracks the pods on the node and can provide the identity. 

UDS requires few small changes - but adds ability to strongly identify the peer and to pass file descriptors
between processes - including shared memory segments that can provide some zero-copy semantics. This works
for containers - not for VMs running on the same host. 

Virtio is probably the best option when the workload needs strongest isolation. Gophers in gVisor provide
a similar capability.

# Zero copy

The best case is a zero copy mechanism - it is the closest to 'monolith' behavior. Avoiding context switch
would also be great - Android Binder and few other 'fiber' mechanisms allows this for example.

[Splice and pipes](https://www.kernel.org/doc/html/next/filesystems/splice.html) and the splice call are 
powerful mechanisms enabling some zero copy. 

Hugetables and shared memory segments, combined with flatbuffers and uring-like queues is another option.

