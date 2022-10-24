package quic

// 2021/04: mostly commented out, using the H3 library directly ( with a smaller patch).
// May need it back for performance.

// 2020/09:
// - still not merged mtls patch for HTTP3,
// - missing push
// - low level QUIC works great !!!

// Modified QUIC, using a hack specific to Android P2P to work around its limitations.
// Also adds instrumentation (expvar)

// v2: if wifi connection is DIRECT-, client will listen on 0xFF.. multicast on port+1.
//     AP: if destination zone is p2p, will use the MC address and port+1 when dialing
//         multiple connections may use different ports - MC is next port. Requires knowing
//         the dest is the AP - recorded during discovery.
//     AP: as server, if zone is p2p, use port+1 an MC.
//     AP: as client, same process - the GW will have a port+1

// v3: bypass QUIC and avoid the hack, create a dedicated UDP bridge.
//     should work with both h2 and QUIC, including envoy.
//     AP-client: connect to localhost:XXXX (one port per client). Ap client port different.
//     Client-AP: connect localhost:5221 (reserved).
//     AP listens on UDP:5222, Client on TCP/UDP 127.0.0.1:5221 and UDP :5220
// Need to implement wifi-like ACK for each packet - this seems to be the main problem
// with broadcast. A second problem is the power/bw.

/*
Low level quic:
- stream: StreamID, reader, cancelRead, SetReadDeadline
          writer+closer, CancelWrite, SetWriteDeadline
-

*/

/*
env variable for debug:
Mint:
- MINT_LOG=*|crypto,handshake,negotiation,io,frame,verbose

Client:
- QUIC_GO_LOG_LEVEL=debug|info|error
*/

/*
 Notes on the mint library:
 - supports AES-GCM with 12-bytes TAG, required by QUIC (aes12 packet)
 - fnv-1a hash - for older version (may be used in chrome), unprotected packets hash
 - quic-go-certificates - common compressed certs
 - buffer_pool.go - receive buffer pooled. Client also uses same
 -

	Code:
  - main receive loop server.go/serve() ->


  Packet:
   0x80 - long header = 1
	0x40 - has connection id, true in all cases for us


  Includes binaries for client-linux-debug from chrome (quic-clients)

  Alternative - minimal, also simpler: https://github.com/bifurcation/mint
  No h2, but we may not need this.
*/

/*
	May 2018: quic uses mint. client-state-machine implements the handshake.

	- without insecureSkipVerify, uses RootCAs, ServerName in x509 cert.Verify(VerifyOptions)
	- either way, calls VerifyPeerCertificate

*/
// tlsconfig.hostname can override the SNI
//ctls.VerifyPeerCertificate = verify(destHost)
//qtorig := &h2quic.RoundTripper{
//	//		RoundTripStart: h2.QuicDialer,
//
//	TLSClientConfig: tlsConf,
//
//	QuicConfig: quickConfig(),
//	// holds a map of clients by hostname
//}
// TODO: initial handshake (WorkloadID, POST for messages, etc)

// TODO: Quick stream has an explicit CancelWrite that sends RST, and Close
// sends FIN.

// H3 spec:
// - CONNECT is not allowed to have path or scheme
// - we should use CONNECT
// - each stream starts with a 'type' (int)
// - Frame format: type(i) len(i) val
// -

// QUIC_GO_LOG_LEVEL

//if streams.MetricsClientTransportWrapper != nil {
//	qrtt = streams.MetricsClientTransportWrapper(qrtt)
//}

// --------  Wrappers around quic structs to intercept and modify the routing using multicast -----------
