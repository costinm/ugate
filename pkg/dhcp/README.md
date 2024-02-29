# DHCP server

For home network and VMs - and experimenting.

Goals:
- act as DHCP4/6 server, using the mesh control plane as source of info for 
   registered MACs/hostnames
- for new hosts, register with the mesh at DHCP time
- for VMs / machines without OS - experiment with automatic provisioning (tftp, https)

Alternative: 
- watch /tmp/leases and update on change
- mount /tmp/leases as a 9P ?

Others:
- https://coredhcp.io/
  - redis plugin, etc. Can be used standalone and extended. 
