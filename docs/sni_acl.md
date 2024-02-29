# SNI routing

SNI routing has access only to the destintion address - there is no authenticated info about the caller.

That means:

- should be restricted with NetworkPolicies and similar firewall rules
  - only allow-listed workloads can connect to the SNI router
  - SNI router should be restricted to only specific workloads as destinations
- source info is lost, NetworkPolicies based on IP or labels can't be used on the destination.
  - however if TLS+auth is used - identity-policies work.
- ideally only exposed/used on local host (for egress) or specific, rare cases on ingress
- avoid exposing on a public address, except for explicitly configured addresses where mTLS all the way to the destination workload is required. This is what (original) Istio multi-network does.

A better option is to tunnel mTLS over H2, where the caller can be authenticated and authorized, and additional metadata is available.

SNI routing is intended for 'legacy' applications and compatibility with the current multi-network Istio infrastructure, while HBONE/MASQUE/etc are developed and adopted.
