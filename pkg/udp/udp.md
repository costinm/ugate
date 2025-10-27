# Protocols over UDP

## TURN/STUN

- data channel packets start with 2 byte channel ID, in 0x40..0x7F range.
- 

# micro Quic 

For encrypted UDP there are few options:

- Wireguard/IPSec - usually at kernel level, but it is possible in user space too.
- QUIC
  - a low-level direct QUIC implementation
  - MASQUE - which uses some CONNECT over QUIC at high level
- WebTransport or WebRTC

A UDP packet has an MTU - there is no fragmentation or ordering, and no ACK.



For a larger 'one way message' - besides a custom protocol we can also use QUIC
packet format. 

