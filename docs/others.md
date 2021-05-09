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


Usage:

```shell

# socks+http proxy
gost -L=:8080


```
