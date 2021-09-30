

Assumptions:
- load tests against the workload in the given CPU/memory alloc shows it is capable
to handle 200 QPS with the required QOS (latency, etc). 
- the service contains workloads in multiple clusters/VMs, with diverse 
CPU allocs.
  
Istio and other client load balancers can handle weighted load distribution, 
but it is extremely hard to prevent distributed clients from creating more 
than 200 QPS on a specific workload. 

The workload's sidecar - may be Istio Envoy or a lighter sidecer or 'native' 
proxyless gRPC - can track exactly how many requests are active - as well 
as the latency and CPU characteristics of the workload.
It is the best place to determine if the workload can handle the load - and 
shed or rebalance the load.

# Identity problem

The main problem preventing Istio sidecar ingress (and alike systems) from 
forwarding the request to another instance

# Cross-replica communication

Workloads may report the load to the control plane - and this can be 
aggregated and distributed to all clients, resulting in a better 
distribution of load. However the process is slow and expensive - it
needs to be spread to all clients.

Having the load sharedacross the backend replicas is more efficient, 
typically the number of endpoints in a service is smaller than the number
of client workload that may call the service. 

A side benefit of creating a cross-replica communication system is that 
it may allow the replicas to exchange other information and implement
things like sharding and stickyness - a request can go to any replica
and be re-forwarded to the correct backend.


