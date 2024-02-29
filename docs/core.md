# Core protocols and services

## DNS

Everything starts with DNS - it is the backbone of Internet and largest discovery server.

Modern DNS (on the internet) is using DNS-SEC - signed records, and DNS-over-TLS/HTTPS for
host-to-resolver OR plain DNS over a secure network (IP-Sec). Unfortunately in many cases 'modern' 
is not used, in particular on home networks.

A 'mesh' DNS should:
- provide extended discovery for pods and metadata (alternative to MDS)
- security
- operate in disconnected mode

## DHCP and CNI-IPAM

Most home networks and routers use dnsmasq - a combined DNS and DHCP server. It creates a 
'database' with the allocs in a file - but it is not very distributed. It is however very small.

Docker uses 02:42:IP_ADDR to FF:FF. (02 means 'local admin')

On a mesh, it would be ideal for devices to use public-key based IP overlay addresses for containers
and apps, and possibly for hosts too, with IPv6 for the core network. Each network segment also
has FE80:: addresses.  

The mesh and DNS/discovery system will need to map pods/containers/devices to hosts and 'via' gateways.

## SAMBA, NFS, WebDAV, S3

Blob storage is commonly used - needs to be secured and discoverable. 

## Prometheus, Jaeger, Zinc, Grafana

That provides:
- a database with key/value based records and '[]float' values
- a blob database keyed by trace-id and with a 'json' body
- log storage with search
- visualisation

## Cert-Manager 

ACME certs can also be managed with CLI tools. 

## Dynamic DNS registration

For home networks. This is also something 'isolated' VMs, laptops, mobile devices should use - 
using the org DNS.





