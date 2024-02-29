# Use in dmesh

All nodes, workloads and services have an overlay IPv6 as well as multiple IPs.

The interface address is usually derived from SHA of the public key.

For network, options are to use the FC::/8 range (not defined, reserved for local), 
or FD::/8, which is specified as 48bit random and 16 bit 'subnet id'.

The 48bit can be derived from the SHA of the fabric public key, subnets may 
define different locations.



# Docs and notes

https://en.wikipedia.org/wiki/Reserved_IP_addresses
https://datatracker.ietf.org/doc/html/rfc6890 Special-Purpose IP Address Registries
https://datatracker.ietf.org/doc/html/rfc5735 - IPv4

0.0.0.0/8 - 'this network'. Can use 0.0.0.0 as source to 'learn the IP'.
10.0.0.0/8
172.16.0.0/12
192.0.2.0/24 - TEST-NET
198.51.100.0/24 - TEST-NET-2
203.0.113.0/24 - TEST-NET-3
192.168.0.0/16
198.18.0.0/15 - benchmark, not routed

100.64.0.0/10 - shared address space https://datatracker.ietf.org/doc/html/rfc6598

224.0.0.0/4 - multicast - a huge space that can be used ! 0xE0.00.00.00

240.0.0.0/4 - future use - 0xF0.00.00.00

169.254.0.0/16 - link local, auto-config (no DHCP allowed). rfc3927
    169.254.169.254 used as MDS. Also DNS in GCP
    'can't be used by any router' - but ok on the bridge
    169.254.169.123 - NTP in AWS
    169.254.169.253 - DNS in AWS
    169.254.1.0 to 169.254.254.255 is the range for random addresses, 0 and 169.154.255.0 are free.
    recommends consistent random generator (MAC?), storing an assigned address, plus ARP probe and announce
    same domain with ARP announce - i.e. bridge

192.0.0.0/24 - IETF assignments
192.88.99.0/24 - 6to4


https://datatracker.ietf.org/doc/html/rfc5156 - IPv6
