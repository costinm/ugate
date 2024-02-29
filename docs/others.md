# Other similar solutions

https://github.com/anderspitman/awesome-tunneling



# MASQUE and QUIC

https://blog.cloudflare.com/unlocking-quic-proxying-potential/

# IPFS / libP2P


# Syncthing


# Tor


# BitTorrent

# [Konectivity](https://github.com/kubernetes-sigs/apiserver-network-proxy.git)

Narrow use case of 'reverse connections' for Nodes (agents) getting calls from the APIserver via proxy,
when the APIserver doesn't have direct connectivity to the node.

gRPC or HTTP CONNECT based on the agent-proxy connection, and gRPC for APIserver to proxy.

``` 
service AgentService {
  // Agent Identifier?
  rpc Connect(stream Packet) returns (stream Packet) {}
}
service ProxyService {
  rpc Proxy(stream Packet) returns (stream Packet) {}
}

Packet can be Data, Dial Req/Res, Close Req/Res
```



# Gost

[gost](https://github.com/ginuerzh/gost/blob/master/README_en.md) provides multiple
integration points, focuses on similar TCP proxy modes.

Includes H2 tunnel, TProxy

Best option: socks5 protocol allows UDP, remote listens
Transport: ssh or h2 -with certificates


Usage:

```shell

# socks+http proxy
gost -L=:8080

gost -L=tcp://:8081/127.0.0.1:5201 http2://127.0.0.1:8080

```

H2:
```text
CONNECT / HTTP/2.0
Host: 127.0.0.1:5201
User-Agent: Chrome/78.0.3904.106


2022/08/09 09:50:49 http2.go:97: [http2] HTTP/2.0 200 OK
Connection: close
Date: Tue, 09 Aug 2022 16:50:49 GMT
Proxy-Agent: gost/2.11.1

```

- auth support admin:pass@addr, addr?secrets=secrets.txt
- URL for the local address - http2, socks5
- Protocols:
  - quic
  - socks5+wss
  - http2 - only tls
  - h2 - supports h2c
  - socks
  - shadowsocks
  - forward:ssh://:2222
  - https
  - http
  - http+tls
- Local protocol:
  - tcp://:LOACLPORT/DEST



gost -L=udp://:5353/192.168.1.1:53?ttl=60
Using UDP-over-TCP, must be socks5
gost -L=rudp://:5353/192.168.1.1:53?ttl=60 [-F=... -F=socks5://172.24.10.1:1080]
Remote UDP - listens on the socks5

Code:
- clean, well separated interface: transport, protocol, mux
- tuntap - direct use of netlink for linux, ip commands for unix
- sockopts - set socket mark, interface  ( -M -I CLI options)
- using tenus project for netlink - can create veth, put them in namespaces.
- using docker/libcontainer for setting routes

Config:
- 'route' - ServeNodes (-L), ChainNodes (-F) - list of URLs
- Retries, Mark, Interface

- The URL has the listener config using options

# OpenZiti


- SDK-based (proxyless) with optional tunneler
- tunneler uses lwip, tun - and the SDK

- WASM openssl repo - for browser zero trust, mTLS over WS
- 'id file' - a json config file. uGate started to use kubeconfig format.

Services are similar with Clusters and K8S Service.

API:
- Options: onContextReady, onServiceUpdate 
- Dial, DialWithOptions -> edge.Conn (net.Conn, CloseWriter, GetAppData, 
SourceIdentifier, TraceRoute(!), Id() uint32, )
- channel.Underlay: Rx, Tx on Message, Identity, Headers


Issues:
- Dial doesn't seem to take context. DialWithOptions has timeout, initial data.
- 

# Netpoll

https://github.com/cloudwego/netpoll

Defines a 'zero copy' model, based on `Next(n) []byte` and Release() intead of Read(buf)
and 

https://github.com/cloudwego/netpoll/blob/develop/nocopy_linkbuffer.go - 
uses unsafe for string/slice

# https://github.com/lesismal/nbio

- for websocket
- nb tls 

# FRP

- golang
- v0 - 
- v1 - over complex, envoy/istio like

Deps:
- socks5 (armon), oidc, beego, kcp-go
- gorilla ws and mux
- pion/stun
- k8s
- quic
- https://github.com/pires/go-proxyproto - ha proxy

# Boringproxy

- ssh 
- golang

# pyjam.as

https://tunnel.pyjam.as/
- requires wg-quic

- wireguard based
- caddy on server side, UDP to the host
- uses 'caddy load' API to dynamically add a route to the local caddy
- single host (good for self-hosting, moderately small VPCs - not high QPS)
- interesting example to automate wg

#  Chisel

- https://github.com/jpillora/chisel
- ssh over ws

# Bore

- rust
- not HTTP proxy - just TCP ports
- may be used with a HTTP proxy like caddy or envoy (with dynamic API)
- control port and ACCEPT 
- single host

# Rathole 

- support TLS, TCP and Noise with keypairs (like wireguard)
- uses control channel
- single host
- faster than frp
- "Use ngix for http forwarding, with rathole as upstream"

# zrok

- based on openziti
- net foundry FE or self-hosted
- Openziti:
  - tunnels for multiple platforms (incl android)
  - mTLS and JWT and other authn
  - no L7 AFAIK
  - Dial and Bind policy - who can provide or use a service
- libsodium based - wasm included, ed25519
- java, c, .net, swift
