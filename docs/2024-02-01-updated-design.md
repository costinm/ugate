# Protocols

Like ambient, the HTTP and TCP separation is restored. 

As the core mesh protocol - I'm moving back to SSH. I want to test an alternative to HBONE (to validate that other 
protocols can be plugged), and I think SSH is a more useful protocol for devices and integrations. 

No longer using Quic or H3 - instead WebRTC and webtransport, as they have more support in browsers and are 'user space'.

Basic support for HBONE/CONNECT is intended for interop with Istio and HTTP-based infra (cloudrun).

Core protocols for listeners:
- SSH - main mesh - on default port
- hbone and hbonec - http handlers configured with the protocol handlers, on default port.
- HTTP, including H2C
- HTTPS, including H2. The listener also accepts TLS routes (SNI).
- MQTT
- SOCKS5
- webrtc
- webtransport

Extended protocols:
- ws
- quic
- h2r
- ipfs



# Routing

Coupling any particular routing or LB with the data plane is not ideal. 

Instead, the data/mesh layer is expected to integrate with a 'metadata system' that may provide information about
any particular destination. DNS is probably the best common option, but K8S, XDS servers and other options exist.

UGate provides a Dialer and RoundTrip API, as well as egress proxy protocols - iptables, socks, http proxy, tun.

The primary addressing is based on FQDN - HTTP, socks, Dial(), http_proxy have an address which is used as the key in
the discovery system or local config to find the metadata.


## IP-based mesh

TUN, iptables only have access to an IP:port - just like Istio sidecars. Based on istio mistakes, this uses a more 
restricted approach:

- dedicated ports - 80, 443, 8080, ... - based on the port we can terminate http, http2 or sniff TLS and extract a FQDN, which will be used instead.
- for any other port, a PTR lookup to the MDS server or the local config should return a FQDN.

Once the FQDN and metadata are available - it will follow the same model as FQDN-based routing.

### XDS discovery

TODO: If an XDS server is configured, it should be used first - expecting ambient or istio records.

### K8S discovery

If running in K8S env, the apiserver can be queried for config (not watched!). A per-node or larger cache should be used. 

### DNS discovery

As a baseline, a secure DNS with DANE, TXT and other records is expected.



## Self-assigned identity

Original design was centered around a self-assigned public key identity. Other protocols use the same model, and it is still
worth exploring. Each workload still gets a unique public key, which is mapped to a IPv6 VIP and FQDN. 

As suffix, ".m", "h.mesh.internal", "h.mesh.example.com", ".m.example.com" could be used. 

When a client uses the 'self-assigned' FQDN or VIP, the following rules are used:
- VIP is mapped to FQDN first.
- local discovery (mDNS, local DNS, etc) are used for the FQDN to get A record. 
- connection will validate the public key directly - treating it as a DANE record.
