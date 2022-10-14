# Performance tracking

sysctl -w net.core.rmem_max=2500000
sysctl -w net.core.rmem_max=10000000

ip link set lo mtu 9000

# Links

https://blog.cloudflare.com/how-to-receive-a-million-packets/
https://www.ibm.com/cloud/blog/achieving-10-gbps-network-throughput-on-dedicated-host-instances

[investigate modifying the kernel's receive buffer size](https://github.com/lucas-clemente/quic-go/issues/2255)

[Quic in libp2p perf](https://docs.google.com/document/d/1JWOpigjvM79OqmNn5Ja_RpuQZGQfIm8QYpeR-5So9Lo/edit#heading=h.r8uwdfmz88yn)

[Quic in syncthing](https://github.com/syncthing/syncthing/pull/7417/files)

https://github.com/marten-seemann/qperf
https://github.com/h2o/quicly/

[Pion perf](https://github.com/pion/sctp/issues/62) - TCP buffer 208k, growing dynamically.

https://arxiv.org/pdf/1706.00333.pdf
int buffer = 4000000;
setsockopt(s, SOL_SOCKET, SO_SNDBUF, buffer, sizeof(buffer));
setsockopt(s, SOL_SOCKET, SO_RCVBUF, buffer, sizeof(buffer));
net.core.rmem_max=12582912
net.core.wmem_max=12582912
net.core.netdev_max_backlog=5000
txqueuelen 10000
9000 MTY, 8972 packet

https://events.static.linuxfound.org/sites/events/files/slides/LinuxConJapan2016_makita_160712.pdf
- many optimizations for UDP
- SO_REUSEPORT
- SO_ATTACH_REUSEPORT_EBPF
- disable source IP validation, auditd, iptables, GRO
- interrupt coalescence

# MTU

Since the main limit seems to be 'packets' at kernel level, larger MTU helps a lot.
QUIC is also good at packing.

Ethernet max is 1500 ( min 576 for v4, 1280 v6 ).
Wifi: 2000
802.11: 7935
Jumbo frame: 9202 "most equipment supports 9000"
64000 limit - segment size (fragmentation)

loopback has mtu 64k 

Quic hardcodes it to 15k - so lo is not a good use

TCP also has offloading and other optimizations.

# Large receive offload

'restricted to TCP'

## Tools

pprof & iperf3 -c localhost -p 15201
go tool pprof pprof http://127.0.0.1:15400/debug/pprof/heap
pprof -http=:8080 /home/costin/pprof/pprof.___3hbone.alloc_objects.alloc_space.inuse_objects.inuse_space.005.pb.gz


docker run uber/go-torch -u http://192.168.3.16:15400/debug/pprof -p -t=30 > torch.svg


wget http://localhost:15400/debug/pprof/trace?seconds=10 -O trace1
go tool trace trace1.log

### Goben 

- go, all CPUs
Baseline send/receive: 16..17Gbps, receive/send: 34Gbps, 


gRPC stack:
- goben -hosts localhost:15201  -tls=false -totalDuration 20s -passiveServer : 4.4Gbps ?
- goben -hosts localhost:15201  -tls=false -totalDuration 20s -passiveClient: 4.6 Gbps


## 2022/09

For localhost tests (or on a very fast network), if iperf3 tests work well but tunnel is slow:

Frame graph showing a lot of time in Read: it means the Write on the other side is delayed
due to lack of flow control ( Window updates).

A lot of time in Write: it means the other side is slowing down at TCP level (flow control), or 
that Read on the other side is slow. 

Hbone initial: 2.18 G
H2 settings in the stack: defaults, hardcoded: initialWindowSize: 64k, initialMaxFrameSize: 16k, max 1M.
Frame size max (real): 16M.

After max frame size: 4.38

gRPC stack: 5.7 (no tls)

## 2022/08


Gost
```shell
# -D = debug
# Remote (PEP, egress GW)
gost -L "http2://:8081?cert=/etc/certs/cert-chain.pem&key=/etc/certs/key.pem" \
    -L :8080 -L h2://:8082 -L socks5://:1080 -L tcp://:5202/127.0.0.1:5201 -L quic://:8083 \
     -L kcp://:8084 -L forward+ssh://:2222 -L ssh://:2223 -L redirect://:15201 -L h2c://:8085 \
     -L ws://:8086 -L wss://:8087 -D

gost -L http2://:8080 -L socks5://:1080

# TCP proxy
gost -L tcp://:8081/127.0.0.1:5201
gost  -L tcp://:8081/127.0.0.1:5202

# H2C proxy

# Socks proxy
gost  -L tcp://:8081/127.0.0.1:5201 -F socks5://127.0.0.1:1080

```

| Test    | Speed  | Notes |
|---------|--------| 
| Baseline | 24Gbps |
| TCP proxy | 23     | Golang
| TCP proxy x2 | 23 | 
| http2c proxy | 3.18   |
| H2  | 3.1() |   TLS |    
| h2c | 6.44 | 
| Socks5  | 7      | Auto-TLS |
| quic | 0.4 | 
| kcp | 0.3 | 
| forward-ssh | 4.45 |
| ssh | 4.56 |
| http1 | 14.3 | CONNECT |

Envoy

| Test    | Speed  | Notes |
|---------|--------|
| Baseline | 33Gbps |  direct
| 1 TCP  | 17 Gbps  |  1 hop, TCP
| hbone  | 7 Gbps | TLS
| hbonec | 11Gbps | no TLS

HBone-go-grpc stack

| hbone  | 6.3 Gbps | TLS
| hbone - boring | 6.24 | 
| hbonec | 10.5 Gbps | no TLS

HBone-go-h2 stack

| hbonec | 3.5
| hbone | 5.0 - reverse 2.5 - large frames

## 2021/04 - iperf tests

Tests over localhost, AMD Ryzen 5 2600.

```iperf3 -c localhost -p PORT```

- Baseline (:5201) : 22Gbps

- One gateway, slice (gate:6011 forwardTo :5201): 19Gbps, 17Gpb(-R)
  
- H2 forward (Alice:6411 -> H2 -> Gate -> :5201): 3.5Gbps
  
- H2 Reverse (Gate:6013 -> H2R -> Alice -> :5201): 1.43Gbps
  This is using a POST created by client, multiplexing client requests with H2.
  
  
- Quic forward, H3 (Bob:6111 -> Quic -> Gate -> :5201 ): 598Mbps
- Quic reverse (Gate:6012 -> Quic Rev -> Bob -> :5201 ): 600Mbps

So far the streams are using H2 or H3 encoding and standard libraries.

Next step: use 'raw' protocol and also test Quiche.

- Quic forward, raw QUIC: 493Mbps

