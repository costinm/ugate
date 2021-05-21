# Notes on DNS and SNI

For TLS, the main routing info is the SNI part of ClientHello.

For compatibility with most clients, it must be a hostname - i.e. subject to:
- max 256 bytes
- each label max 63B
- start with letter, contains letter and digits and "-" (not in first or last).


Standard convention for IDN is to start with "xn--" prefix to indicate UTF name.
IPFS/LibP2P also have the convention to start with a byte identifying the syntax of the rest.
Starting with a 'type' also makes sure it starts with a letter, and the rest can be
hex or base32.

DNS allows _ and other binary characters - but this is unlikely to work with TLS clients
and servers.

https://github.com/multiformats/multibase defines some prefixes to identify the base
encoding of a string. We also want to use a tag to identify what kind of address it is. 
They use:
- 'f' = hexadecimal
- 'b' = base32 (lowercase) - not using 0, 1, 8, 9 or -

For ugate needs, the first label of the domain is used to encode the node ID or dest.
Since this is a mangled name, we can also use the size for the identifier and presence
of dashes.

We may support:
- ugate 32-byte KEY, base32 ( 32 * 8 / 5) = 52 chars, can't be encoded in hex. This is 
  the primary format, since it can encode a ED25519 public key or SHA.
- ugate 8-byte ID, hex: 16hex digits = 16 chars. Base32 is 13 chars, not worth it.
- ipv4 - simplest is 'i80-10-1-1-2'
- ipv6 - simplest is 'p80-fd--01' ( : replaced with -) - will have double dash or > 3 dashes
- alternative - all hex, no dashes: 12 chars for IPv4 and port, 36 chars for IPv6 and port

Since first char must be a letter, and hex and base32 can start with digit, prefixes are used:
- u - for  32-byte base32 ID
- f - for  8 byte hex 
- a - for hex IP and port, len determines ipv4 vs ipv6
- i - for port-ipv4
- p - for port-ipv6 - max is 36 bytes (16 + 2), leaving 63-36-1=26bytes

The ugate primary port is expected to handle 'BTS'-style communication, with the first 
component used to identify the mesh address. An optional prefix "nXXX-" may identify 
the network. We need to keep the net id short - max 24 bytes.

For short namespace/service, we can also use:
- s[PORT]--SVC-NS - if len > 63, the truncated SHA with prefix -- can be added as suffix
- s[PORT]-[NUMBER]-SVC-NS for stateful sets

