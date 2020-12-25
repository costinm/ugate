For testing, can also be used as a swap-in for istio proxy.

Example:

```
PROXY_GROUP=costin INBOUND_PORTS_INCLUDE=3001,5201 OUTBOUND_PORTS_INCLUDE=5201 bash -x ./iptables.sh
iptables-save -c| grep ISTIO
tcpdump -n -i any port 15006

# Turn off
PROXY_GROUP=costin INBOUND_PORTS_INCLUDE=- OUTBOUND_PORTS_INCLUDE=- bash -x ./iptables.sh

```

Iperf3 should show line speed with and without proxy.

Example numbers (Gbps):

``` 
iperf3 -c 10.1.10.228
Raw: 28.4 

Direct proxy:
iperf3 -c 10.1.10.228 -p 3001
21.2

IPtables enabled: 20.7 (outbound capture only)



```
