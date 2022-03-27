# Low level protocols used


## H3 frames


H2 frame format - 9 byte header:
- Len(24)
- Type(8)
- Flags(8)
- StreamID(32)

H2 int format, used in HPACK and QPACK: mix of varint and 'N-bit prefix'. 
Short ints are 1 B (with few bits for flags), rest are varint, first byte uses 1111... for the N bits.

QUIC int use first 2 bits to encode length ( 1,2,4,8)

 

## Websocket frames

After handshake, WS sends frames including at least 2B overhead for server originated,
and 2B + 4B(mask) for client:

- FIN
- frame type - ping, ping, text, binary, cont, close
- payload - 1B or 3 B
- 

# WebRTC 

In future it may use QUIC - currently UDP+SCTP. Best protocol for interop with web browsers, 2-way.
