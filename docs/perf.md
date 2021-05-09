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

