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

 
