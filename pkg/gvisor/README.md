# Integration with gVisor TCP/IP stack

Original attempt used a number of patches to netstack (before it was merged back into
gvisor repo). I proted them to the new repo, but it is hard to maintain and never
got the time to push them up.

The patches were mainly to capture all and support 'transparent' mode for UDP.

gvisor-tap-vsock project solves this - and much more, so I will gradually move
to using it. 

## Raw and packet UDP 

Return the full packet - but after all the gvisor processing. Packet cooked and raw seem the same.

Alternative is to use NIC.DeliverNetworkPacket only for TCP, and process the packet directly for UDP.
This may also avoid few mem copy.


## UDP sending with 'original src'

- add SpoofedAddress to stack/packet_buffer
- use it in ipv4/ipv4.go WritePacket, which adds the IP header

- From added to WriteOptions
- udp/endpoint.go - sets localAdd and localPort. Modify internal sendUDP to add SoofedAddress

## Capture all

- stack/transport_demuxer - use port FFFF at the end, if no other port matches
- same can be done with iptables now - not clear if only REDIRECT or TPROXY too

TODO: should use Istio magic port, and a flag to enable


# gvisor-tap-vsock

- client interface for 'expose' (/services/forwarder/expose, json) local to remote
- tap package defines a VirtualDevice (NewSwitch)
- includes some ssh forwarding code
- includes DHCP, DNS, tcp and udp forwarding

- 'vsock' transport using mdlayhervsock
  - VM to hypervisor or Host machine - AF_VSOCK (http://wiki.qemu-project.org/Features/VirtioVsock)
  - firecracker/others map it to AF_UNIX on host
  - virtio based - tx/rx queue, but exposed as socket
