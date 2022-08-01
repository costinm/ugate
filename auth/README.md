# Auth package 

Dependency free collection of helpers for mesh authentication and provisionig, for client and server.
Provides helpers for client and server TLSConfig, and implements basic mesh authentication.

Usually authen/authz are focused on identity verification and access control using credentials only, 
however auth can provide far more value.

This package combines auth with 'bootstraping' - finding credentials AND basic metadata about
the workload and its environment. It also covers tracking basic peer information - which is
critical for peer authentication but also provides additional metadata about the peer.


```go


```

Features:
- dependency free implementation of Kubeconfig
- bootstrap using platform data - workload certs, JWT tokens, etc.
- basic deps-free OIDC parsing for JWT tokens
- webpush VAPID JWT support and subscriptions
- support Webpush encryption and decryption
- basic STS and MDS support (primarily for GCP and envoy exchanges)
- basic JWT and OIDC parsing

# Dependencies and plug-ins

- ConfStore abstracts loading/saving of the objects used for auth - file, 
remote call, k8s, etc

- GetCertificateHook - for integration with CA or SDS servers. 

# Config

1. Attempt to Load certificates:
- well-known files - currently using K8S workload identity. 
- istio old location
- current dir
- home dir

2. Attempt to load a k8s config

3. Load 'acme' style DNS certs if present (for handling DNS certs)

