This module implements a bi-directional connection using HTTP/2.

# Client to server (normal H2)

Streams are sent using CONNECT or /dm/HOSTNAME:port (TODO: or /ipfs/ID ). 

Client side: can send FIN by closing the In (request.Body or pipe). 

Server side: due to limitations on the server library, it is not possible to 
close the server->client stream until the client->server has been finished.

Status (FIN vs RST) is indicated in an X-Close trailer.

# Server to client (H2R)

Client opens a connection and does an intial handshake to identify (using JWT if mTLS
is not available). Then client open a POST /h2r/ request, where the roles are switched.

In this mode the server creates a HttpClient on the POST stream an makes RoundTrips,
while client calls handleCon and dispatches any request received.

Same limitations as client-to-server on FIN - in this case the client (acting as H2R server)
can't send FIN until the other end is closed.

# Future plans

There are 3 ways around the 'FIN' problem:

- further chunk the stream, adding frames - that allow server to send CLOSE. This is 
  similar with gRPC streamed connections - could use the same format, effectively making
  the TCP connection equivalent with a 2-way gRPC stream.
- fork the h2 library to add an extra method allowing CloseWrite - without full close
- use the low-level framing protocol - which may also allow avoiding some of the overheads
associated with H2 semantics.
  
For QUIC, since the library is forked anyways to expose internal methods - proper CloseWrite 
is used.
