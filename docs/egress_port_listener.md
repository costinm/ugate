K8S:
API_SERVER/api/v1/namespaces/%s/pods/%s/portforward
Forwards a local port to the pod, using SPDY or Websocket.

Docs and other options:
https://blog.ston3o.me/how-to-expose-local-server-behind-firewall/
- OpenVPN - easy to setup docker container
- upnpc
- tor
  ngrok - free 40 con/min
  pagekite - py, $3/month
  bleenco/localtunnel (go)
  localtunnel/localtunnel (js)
  yaler - commercial
  inlets / rancher remote dialer

socks bind standard - not commonly implemented

ssh -R remote_server_ip:12345:localhost:12345
- multiplexed over ssh TCP con, flow control per socket


```
			byte      SSH_MSG_CHANNEL_OPEN
      string    "forwarded-tcpip"
      uint32    sender channel

			uint32    initial window size
      uint32    maximum packet size

			string    address that was connected
      uint32    port that was connected

			string    originator IP address
      uint32    originator port
```

concourse TSA - uses ssh, default 2222
'beacon' is effectively using ssh command to forward ports
"golang.org/x/crypto/ssh"
https://github.com/concourse/tsa/blob/master/tsacmd/server.go

Rancher 'Reverse Tunneling Dialer' and 'inlets':
- use websocket - no multiplexing.
- binary messages, using websocket frames
