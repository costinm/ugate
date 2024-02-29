# Simplified design

## Local Protocols

- SOCKS5 - shadowsocks compatible
- TUN - for compat with Android
- TPROXY - for Istio ztunnel compat
- Iptables - fallback
- HA Proxy protocol

Some inter-process communication between local containers, vm or processes is also
important, doesn't need encryption and can avoid memcopy and serialization:

- Binder
- flatbuffers ? 
- UDS 

## Remote protocols

- HBone - for compat with Istio, browsers (CONNECT)
- WebRTC  

## Firewall bypassing

Use standard TURN servers - with authentication and encryption.
This should work with browsers out of box, and may share infra. 

Why: well established protocols, secure enough for browsers and RTC

As a 'remote gateway' server, it needs to listen on the Gateway ports and initiate
WebRTC to the behind-firewal servers. The goal is to avoid permanent idle connections
and share any firewall holes for UDP and TCP. 
