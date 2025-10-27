
# Gateway support 

This uses an interesting control plane design: a watcher for K8S resources that goes and 
connect to each caddy pod, pushing config - instead of watching.

I think a simpler and safer deployment would be to have the k8s watcher as a sidercar or per-node service - or as a caddy module.

This is not so different from the Istio model of long-lived connections - CP can push
as fast and maintain the same connections. But in a low-change environment the CP
 can timeout the connections.

TODO: use it with a local caddy.

## Rant

Modern computers - and phones - are very powerful, with lots of CPUs and memory. We run
VMs and containers to better pack workloads and use the hardware more efficiently - but
it is common to forget the real hardware and get too attached to common abstractions
like VMs or pods with 1 CPU/1G RAM that we use for tests and demos.

For a machine with 32 cores - or 200 cores - and man TB of NVMe disks attached on fast PCIe
lanes - it is absolutely ok to replicate the entire K8S config and cache as many configs
as you may want - and to run an entire APIserver, Istiod and Gateway dedicated to that
machine. 

It is useful to have a read-only replica, and obviously be careful about private keys and
secrets - the local Istiod and K8S should not replicate that - but the regular configs are
not very secret.

Caddy approach is to use a Gateway controller that is pushing configs to all detected servers.
The servers have their own cache, and can restart with a good config even if APIserver or 
controller are down. 

## Signed configs for K8S

- each node is assigned a private key / identity
- each namespace is assigned a private key / identity
- APIserver - or some other system - signs a token allowing a node to run workloads for a namespace
- labels can be set for affinity - or APIserver scheduler can do provide the delegation tokens.

With this in place, the nodes can run the workloads in that namespace, and prove that
the endpoint and pod identity without requiring the APIserver to be alive. 

Endpoints signed by the node, plus the 'namespace delegated to node' can be discovered 
with node-to-node mesh communication.



# Build

Typical is down with xcaddy and a lot of 'with' - but I think it's cleaner with a main and 
imports.  Also allows nice debug and breakpoints.


# Module model

Caddy does a pretty good job with the modules - my preference is a bit different but very close.

Each module is associated with a Golang struct, and can be loaded from a json config.

In caddy, there is a top level "apps" key, and each key under is the name of a module.
Caddy will load the json and unmarshall it to a new instance of the 'module'.

Once the instance is created, it may implement different interfaces which will be called.

Caddy goes deeper: a top module may also use sub-modules, using the same model. The sub-modules
are not special - but the modules that include sub-modules are (host modules in caddy terms).
This works by using a json.RawMessage

Caddy adds a caddy:"namespace=foo" tag - and has a more complex logic in ctx.LoadModule, but 
ultimately it reads the raw data just like a regular module.

"admin" and "logging" are the other top levels, less generic.

Core interface is Start() and Stop() (both return error - but no Context - the context is passed
 in the Provision(caddy.Context) optional interface)

caddy.RegisterModule(MyModule{}) is used for registering with the config, expected to implement
CaddyModule() returning an ID and a New function.


# Bridging the modules

I like a simpler, flatter config - more like K8S, where everything is a CRD, and 'host' controllers load the resources they need on demand (and maybe watch or get notified of 
granular changes instead of full reload).

I also like 'zero deps' - a module should not depend on Caddy internals.
To be fair - K8S controllers also depend quite a bit of K8S SDK internals and registry.








