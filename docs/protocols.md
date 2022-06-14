# Low level protocols used

UGate primary/target protocol is H3 - it is most flexible and most likely the future.

One of the goals of a gateway is to support multiple protocols, for interop and 
connectivity.

A brief summary of the low levels of each protocol and overhead.

## H2/H3 frames

H2 frame format - 9 byte header:
- Len(24) - 16M chunks
- Type(8)
- Flags(8)
- StreamID(32)

H2 int format, used in HPACK and QPACK: mix of varint and 'N-bit prefix'. 

Short ints are 1 B (with few bits for flags), rest are varint, first byte 
uses 1111... for the N bits.

QUIC int use first 2 bits to encode length ( 1,2,4,8)

HTTP uses compressed headers, but can be turned off in custom protocols.

Core benefits:
- broadly adopted and many implementations
- multiplexed - less TCP/TLS overhead
- flow control for each stream


## Websocket frames

After handshake, WS sends frames including at least 2B overhead for server originated,
and 2B + 4B(mask) for client:

- TYPE(1B): FIN, frame type - ping, pony, text, binary, cont, close
- payload len - 1B or 3 B

Client frames must have a 4B 'mask'

WS is not multiplexed - for HTTP/1.1 upgrade it is ideal for multiplexing H2.

If protocol is H2 already - it's just a stream.

# WebRTC 

In future it may use QUIC - currently UDP+SCTP. Best protocol for interop with web 
browsers, 2-way.

- like quic, low level frames and implementations duplicate TCP, include flow control
- 
