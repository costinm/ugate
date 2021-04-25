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
//	//		Dial: h2.QuicDialer,
//
//	TLSClientConfig: tlsConf,
//
//	QuicConfig: quickConfig(),
//	// holds a map of clients by hostname
//}
// TODO: initial handshake (ID, POST for messages, etc)


const useNativeH3 = false



// TODO: Quick stream has an explicit CancelWrite that sends RST, and Close
// sends FIN.

// H3 spec:
// - CONNECT is not allowed to have path or scheme
// - we should use CONNECT
// - each stream starts with a 'type' (int)
// - Frame format: type(i) len(i) val
// -
//func (ugs *QuicMUX) DialStreamRaw(ctx context.Context, netw string, addr string, meta http.Header) (*ugate.Stream, error) {
//	s, err := ugs.s.OpenStreamSync(ctx)
//	if err != nil {
//		return nil, err
//	}
//
//	str := ugate.GetBufferedStream(s, s)
//	str.In = s
//	str.Out = s
//
//	// Request Cancellation:
//	// This go routine keeps running even after RoundTrip() returns.
//	// It is shut down when the application is done processing the body.
//	reqDone := make(chan struct{})
//	go func() {
//		select {
//		case <-ctx.Done():
//			s.CancelWrite(quic.ErrorCode(0x10c))
//			s.CancelRead(quic.ErrorCode(0x10c))
//		case <-reqDone:
//		}
//	}()
//
//	// Use the stream buffer to encode. To avoid another buffer we could write starting at position 8, and
//	// back-fill the type and length
//	headerBuf := &bytes.Buffer{}
//	enc := qpack.NewEncoder(headerBuf)
//	EncodeHeader(enc, ":url", "test")
//
//	// Send headers, wait for headers
//	for k, v := range meta {
//		// TODO: filter http-specific headers, to avoid RoundTrip having to create a new map
//		for _, vv := range v {
//			EncodeHeader(enc, k, vv)
//		}
//	}
//	b := &bytes.Buffer{}
//	// 1 is h3 normal start. This is a simplified stream-only encoding.
//	quicvarint.Write(b, 0x10)
//
//	quicvarint.Write(b,uint64(headerBuf.Len()))
//	b.Write(headerBuf.Bytes())
//	_, err = str.Write(b.Bytes())
//	if err != nil {
//		return nil, err
//	}
//
//	// Wait for a response header - this is different from regular HTTP libraries,
//	// which start sending the body because POST will send response only after the
//	// body has been received.
//
//	// TODO: use the buffer reader to read eagerly, avoid allocs
//	fr, err := parseNextFrame(str)
//	if err != nil {
//		return nil, err
//	}
//	hf, ok := fr.(*headersFrame)
//	if !ok {
//		return nil, errors.New("unexpected frame")
//	}
//
//	headerBlock := make([]byte, hf.Length)
//	_, err = io.ReadFull(str, headerBlock)
//	if err != nil {
//		return nil, err
//	}
//
//	dec := qpack.NewDecoder(nil)
//	hdrs, err := dec.DecodeFull(headerBlock)
//	if err != nil {
//		return nil, err
//	}
//
//	for _, k := range hdrs {
//		str.OutHeader.Add(k.Name, k.Value)
//	}
//
//	// TODO: unlike std H3, the rest of the stream is just data. We don't need
//	// the extra framing or trailers. We could modify the buffered writer to
//	// 'pack' the result, but too many allocs.
//
//	return str.Stream, nil
//}

//func EncodeHeader(enc *qpack.Encoder, k string, v string) {
//	enc.WriteField(qpack.HeaderField{Name: k, Value: v})
//}

// InitQuicClient will configure h2.QuicClient as mtls
// using the h2 private key
// QUIC_GO_LOG_LEVEL
//func InitQuicClient(h2 *auth.Auth, destHost string) *http.Client {
//
//	qrtt := RoundTripper(h2)


	//if streams.MetricsClientTransportWrapper != nil {
	//	qrtt = streams.MetricsClientTransportWrapper(qrtt)
	//}

	//if UseQuic {
	//	if strings.Contains(host, "p2p") ||
	//			(strings.Contains(host, "wlan") && strings.HasPrefix(host, AndroidAPMaster)) {
	//		h2.quicClientsMux.RLock()
	//		if c, f := h2.quicClients[host]; f {
	//			h2.quicClientsMux.RUnlock()
	//			return c
	//		}
	//		h2.quicClientsMux.RUnlock()
	//
	//		h2.quicClientsMux.Lock()
	//		if c, f := h2.quicClients[host]; f {
	//			h2.quicClientsMux.Unlock()
	//			return c
	//		}
	//		c := h2.InitQuicClient()
	//		h2.quicClients[host] = c
	//		h2.quicClientsMux.Unlock()
	//
	//		log.Println("TCP-H2 QUIC", host)
	//		return c
	//	}
	//}

	//return &http.Client{
	//	Timeout: 5 * time.Second,
	//
	//	Transport: qrtt,
	//}
//}

//var (
//	// TODO: debug, clean, check, close
//	// has a Context
//	// ConnectionState - peer cert, ServerName
//	slock sync.RWMutex
//
//	// Key is Host of the request
//	sessions map[string]quic.Session = map[string]quic.Session{}
//)

//func (ugs *QuicMUX) acceptStream(ctx context.Context, s quic.Stream) {
//	str := ugate.GetBufferedStream(s, s)
//	str.In = s
//	str.Out = s
//
//	// TODO: catch errors, logs
//
//	pt, err := quicvarint.Read(str)
//	l, err := quicvarint.Read(str)
//
//	headerBlock := make([]byte, l)
//	_, err = io.ReadFull(str, headerBlock)
//	if err != nil {
//		return
//	}
//
//	dec := qpack.NewDecoder(nil)
//	hdrs, err := dec.DecodeFull(headerBlock)
//	if err != nil {
//		return
//	}
//
//	for _, k := range hdrs {
//		str.Header().Add(k.Name, k.Value)
//	}
//	log.Println("Quic stream", pt, str.Header())
//
//	str.PostDialHandler = func(conn net.Conn, err error) {
//		// Use the stream buffer to encode. To avoid another buffer we could write starting at position 8, and
//		// back-fill the type and length
//		headerBuf  := &bytes.Buffer{}
//		enc := qpack.NewEncoder(headerBuf)
//
//		// Send headers, wait for headers
//		for k, v := range str.OutHeader {
//			// TODO: filter http-specific headers, to avoid RoundTrip having to create a new map
//			for _, vv := range v {
//				EncodeHeader(enc, k, vv)
//			}
//		}
//		b := &bytes.Buffer{}
//		// 1 is h3 normal start. This is a simplified stream-only encoding.
//		quicvarint.Write(b, 0x10)
//
//		quicvarint.Write(b,uint64(headerBuf.Len()))
//		b.Write(headerBuf.Bytes())
//		_, err = str.Write(b.Bytes())
//		if err != nil {
//			return
//		}
//	}
//
//	// Wait for a response header - this is different from regular HTTP libraries,
//	// which start sending the body because POST will send response only after the
//	// body has been received.
//
//	// TODO: use the buffer reader to read eagerly, avoid allocs
//
//
//	// TODO: unlike std H3, the rest of the stream is just data. We don't need
//	// the extra framing or trailers. We could modify the buffered writer to
//	// 'pack' the result, but too many allocs.
//
//	ugs.streamHandler.Handle(str)
//
//	s.Close()
//}


//// Special dialer, using a custom port range, friendly to firewalls. From h2quic.RT -> client.dial()
//// This includes TLS handshake with the remote peer, and any TLS retry.
//func (h2 *H2) QuicDialer(network, addr string, tlsConf *tls.Config, config *quic.Config) (quic.EarlySession, error) {
//	udpAddr, err := net.ResolveUDPAddr("udp", addr)
//	if err != nil {
//		log.Println("QUIC dial ERROR RESOLVE ", qport, addr, err)
//		return nil, err
//	}
//
//	var udpConn *net.UDPConn
//	var udpConn1 *net.UDPConn
//
//	// We are calling the AP. Prepare a local address
//	if AndroidAPMaster == addr {
//		//// TODO: pool of listeners, etc
//		for i := 0; i < 10; i++ {
//			udpConn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
//			if err != nil {
//				continue
//			}
//			port := udpConn.LocalAddr().(*net.UDPAddr).Port
//			udpConn1, err = net.ListenMulticastUDP("udp6", AndroidAPIface,
//				&net.UDPAddr{
//					IP:   AndroidAPLL,
//					Port: port + 1,
//					Zone: AndroidAPIface.Name,
//				})
//			if err == nil {
//				break
//			} else {
//				udpConn.Close()
//			}
//		}
//
//		log.Println("QC: dial remoteAP=", addr, "local=", udpConn1.LocalAddr(), AndroidAPLL)
//
//	}
//
//	qport = 0
//	if udpConn == nil {
//		udpConn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
//	}
//
//	log.Println("QC: dial remote=", addr, "local=", udpConn.LocalAddr(), AndroidAPMaster, AndroidAPLL)
//	quicDialCnt.Add(1)
//
//	cw := &ClientPacketConnWrapper{
//		PacketConn: udpConn,
//		addr:       addr,
//		start:      time.Now(),
//	}
//	cw.useApHack = h2.Conf["p2p_multicast"] == "true"
//	if udpConn1 != nil {
//		cw.PacketConnAP = udpConn1
//	}
//	qs, err := quic.Dial(cw, udpAddr, addr, tlsConf, config)
//	if err != nil {
//		quicDialErrDial.Add(1)
//		quicDialErrs.Add(err.Error(), 1)
//		udpConn.Close()
//		if udpConn1 != nil {
//			udpConn1.Close()
//		}
//		return qs, err
//	}
//	slock.Lock()
//	sessions[addr] = qs
//	slock.Unlock()
//
//	go func() {
//		m := <-qs.Context().Done()
//		log.Println("QC: session close", addr, m)
//		slock.Lock()
//		delete(sessions, addr)
//		slock.Unlock()
//		udpConn.Close()
//		if udpConn1 != nil {
//			udpConn1.Close()
//		}
//	}()
//	return qs, err
//}

// --------  Wrappers around quic structs to intercept and modify the routing using multicast -----------

