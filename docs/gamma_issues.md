# GAMMA issues and alternatives

## Background 

GAMMA is a workgroup attempting to apply the K8S Gateway API and model for 'mesh'.

The term 'mesh' is defined in a different way by each product - but generally
in K8S context refers to policies applied on the communication between workloads and
Services. 

HttpRoute is the central API. The Gateway API abstracts a reverse proxy / load balancer
that may route to backends and enforce policies.

A very important decision is that the Gateway implementation may choose to run an
in-cluster deployment - or may use external multi-tenant infrastructure.

In many cases it is far more efficient to share resources - if one namespace
needs few 100 qps it is overkill to allocate even 1 CPU - and in most cases
due to replication even more will be wasted. Sharing a gateway across clusters
or even tenants can result in lower costs and higher performance, and may
be easier to manage.

There are also cases where a dedicated deployment is needed - both cases are allowed.

A Gateway is usually associated with a custom DNS domain and external IP - but 
implementations also allow using VPC IPs - or may create K8S Services and use cluster IP.
It is possible to automate the DNS registration with external DNS servers as well
as provisioning of ACME or other public certificates.

An L7 request has a number of steps (for https):
- client DNS resolution - in most cases in K8S this results in a unique 'cluster IP',
 but for internet gateways it is more common to have shared IPs
- a TCP connection is opened to the IP - this is handled by L4 routing, and the CNI implementation in K8S handles
  the cluster IPs
- SNI with same hostname is sent to the IP, and server can use that to lookup and send a cert
- server sends back certificates for the SNI, and client checks - typically matching the hostname/SNI, or using a control plane or config to find the expected workload identity (spiffee)
- after ALPN handshake, the request is processed and "Host" header is matched.
- based on host, different routes and policies are applied.
- the request is forwarded to one of the backends.

# ClusterIP and Service limitations

GAMMA is currently defined as associating HttpRoute with a Service - which has 
a cluster IP and sometimes multiple ports. Implementations may choose 
how to implement this - but since the cluster IP and the Service were 
previously handled by CNI as L4 services, some form of interception or redirection
for the service is needed. The API is pretty much forcing this - there is no 
provision for a client to get a different IP in the DNS resoultion or use the
previous L4 routing rules. Client must use the Service name (service.namespace.svc.cluster.local),
will get the cluster IP assigned by K8S and handled by the CNI layer. 

That works fine for Istio - either sidecar or ambient - because interception is the 
core functionality. But it is not clear this complexity is the best option, and it forces all 
potential clients to also use Istio. 

Running a full gateway with 100s of features next to each worklod has a very large
cost (operations and resources), so Istio ambient is moving to  model where 
the L7 features are handled by "Waypoints", which are shared gateways currently with
namespace granularity.

However due to the GAMMA semantics - some interception to re-purpose the Service IP is still required.


A better approach for many use cases that GAMMA supports is to simply use the
real Gateway API - with an 'internal' class, as well as using a new hostname. While this
may require clients to be configured to use a new name - for new applications 
it is not a huge burden. The old name with 'svc.cluster.local' is too tied with
the ClusterIP, unique service IP - and the old IP are tied to the workings of the CNI.

Istio has put very high priority on the 'user should never have to make changes
in their app config' - and Ambient puts an even higher priority on 'you should
not break any existing app'. Both have huge price - and many corner cases.

A developer should be able to make his own choice - and having a simpler architecture
and lower risk and costs may be worth making few config changes for a lot of users.

There are additional benefits for this approach:
1. the new DNS may use a managed DNS server and be visible in the entire VPC
2. user can pick whatever names they want
3. if they can use a real domain and with ACME DNS channenge they can even get public certs
4. For HTTP fewer IPs are used - even one per region
5. Same multi-tenant servers that are used for regular gateways can be used.

All this can be done today using the 'real' Gateway API - for the price of a small
config change and using a name chosen by the user instead of the fixed K8S naming conventions.

## Native TLS

More and more applications are using client libraries that default to TLS+JWT - for any
such application interception is going to fail to apply any L7 policy. 

The usualy examples in Istio and gamma show apps making http:// requests - and 
the intercepting proxy applying policies. It works because after cluster IP is intercepted
we can parse the http request and see the path/headers - but it doesn't work
if the client originates a https or TLS connection.

In the early days it was very common for apps to use plain text in the internal VPC,
and probably still is - but tools like CertManager make it very easy for apps to 
get certs and frameworks and libraries are starting to be more security aware.

For any app doing native TLS, GAMMA is not practically implementable. Using 
an internal gateway works - if it gets proper name and certificate that is compatible
with the client libraries, which generally use DNS verification ( spiffee requires
the apps to use XDS to get the server workload identity - proxyless can do this, but it
is very rare to have real Gateways use spiffe since it requires all clients to be in the mesh)

# Recommended uses for GAMMA

## Client apps that can't be changed/reconfigured and use http://

If the application can't be changed or reconfigured to use a real Gateway where
routes are applied - we must be able to apply the HttpRoute on the path from 
client originating the request to the cluster IP of the Service. As mentioned
above, that typically requires interception in a sidecar or per node - before the 
packets get encrypted by CNI. 

If the application can be reconfigured to use a different address - using a real
gateway avoids all this complexity and has far more flexibility.

It would be best to define an internal Gateway anyways - so future versions 
of the app or clients not running Istio/etc ( not in K8S or env supporting interception)
can still use the service with the routes/policies.

If the client app is using https:// - mesh implementations using interception can't 
do much against a proper implementation, only option is to point to the 
internal Gateway who can terminate TLS and apply the policies.

## Pods with local host affinity

Most Gateway implementations run Pods or use external shared infrastructure.
In performance-critical cases it is useful to co-locate related pods using 
affinity - the pods would use node-local paths with higher bandwidth and lower
latency then ethernet/fiber. 

Generally encryption between 2 containers on same machine is not required (or 
very useful) - if the node is compromised it can bypass the protection. Encryption
is important for over-the-wire and against sniffing or MITM - it does not 
protect against kernel/root on either end.

K8S provides Service options to prefer or require same-node - and usually the
apps are written to take advantage and not use https:// so are compatible with 
interception-based implementations.

In such case a GAMMA implementation can be better, if it is aware of same-node.
Ztunnel has this capability - but only for L4. Istio sidecars currently don't.
I'm not aware of any mesh that can do this yet at L7 - except maybe a per-node
Waypoint that is not very feasible. 

## Proxyless - or 'native' implementations

Istio and gRPC have used the term 'proxyless' to describe native implementations
of the policies, where libraries or framework code are used to get the configs
and implement them directly. Routing and load balancing policies would be on 
the client side, authz on the server side.

If client and server are on the same node this can have a huge impact, since 
local communication doesn't require encryption and is significantly faster 
then over-the-wire. Even if the L7 LB was per-node, proxyless has one less hop - 
and running per-node L7 ILB has operational and security costs.

Proxyless can see the internal Gateways and Routes and directly implement the configs.

While benefits are clear, there are costs too:
- feature gaps - a fully featured Gateway has a lot of features and additional APIs
- security risks - the external gateways can run on separate nodes and accounts/teams
- if some sensitive policy, in particular security, is missing there are risks associated with the gap.
- language and protocol support is limitted - gRPC supports few languages, other protocols are currently Go-only
- code/binary size increases 
- complexity and bugs in the framework require more frequent builds/updates, sometimes the mesh implementation is order of magnitudes larger than the app code.

A proxyless implementation can be used with GAMMA - it could also be used with an 
internal Gateway, if the control plane decides it can bypass the gateway.

It is critical for implementations to be aware of the server capabilities, in 
particular on Istio-Ambient like implementations. If the server has a sidecar or
is a proxyless implementation AND all policies are in the supported set, it can
bypass the Waypoint or Gateway. Otherwise it should still be routed to the middle
proxy, and NetworkPolicy should restrict direct communication with the backend.

## WASM and Agents

One option for the proxyless issues is to have a Gateway implementation in WASM
or as a per-node agent - but written as a non-proxy API with some integration with 
the framework. 

For example the http or gRPC library would make a UDS or local call to a sidecar 
or per node agent with the hostname, path, headers - and receive back the destination
IP and metadata (expected CA, what SNI to use, if it is on the same node or requires TLS).

The agents have costs too - and it is useful to have them local. Making multiple
roundtrips can have a higher cost then a proxy.

A WASM implementation can be a middle ground too and can be linked with the app like
a proxyless implementation - but would work with more languages and protocols. 

This has the same limitations and risks as proxyless, plus 1-2 additional same-node 
RTT.  
