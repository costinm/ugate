package ugate

// HBone for compatibility with Istio


// MuxDialer creates an association with a node and returns a mechanism to
// make roundtrips. This is focused on HBone-like connections - but the mux
// can make any HTTP requests as well.
//type MuxDialer interface {
//	// DialMux creates a bi-directional multiplexed association with the node.
//	// The node must support a multiplexing protocol - the fallback is H2.
//	//
//	// Fallback:
//	// For non-mesh nodes the H2 connection may not allow incoming streams or
//	// messages. Mesh nodes emulate incoming streams using /h2r/ and send/receive
//	// messages using /.dm/msg/
//	DialMux(ctx context.Context, node *MeshCluster, meta http.Header,
//			ev func(t string, stream util.Stream)) (http.RoundTripper, error)
//}


// Implements HttpClient, HttpDoer, etc from multiple packages (connect, k8s, etc)
// MeshCluster represents a single destination - hostname will not be used to connect,
// but will be sent. For example the cluster can be an egress gateway or ztunnel.
//func (c *MeshCluster) Do(req *http.Request) (*http.Response, error) {
//	return c.RoundTrip(req)
//}

//// DoRequest will do a RoundTrip and read the response.
//func (c *MeshCluster) DoRequest(req *http.Request) ([]byte, error) {
//	var resp *http.Response
//	var err error
//
//	resp, err = c.RoundTrip(req) // Client.Do(req)
//	if Debug {
//		log.Println("DoRequest", req, resp, err)
//	}
//
//	if err != nil {
//		return nil, err
//	}
//
//	defer resp.Body.Close()
//	data, err := ioutil.ReadAll(resp.Body)
//	if err != nil {
//		log.Println("readall", err)
//		return nil, err
//	}
//	if len(data) == 0 {
//		log.Println("readall", err)
//		return nil, io.EOF
//	}
//
//	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
//		return nil, errors.New(fmt.Sprintf("kconfig: unable to get %v, status code %v",
//			req.URL, resp.StatusCode))
//	}
//
//	return data, nil
//}

//func (c *MeshCluster) RoundTrip(req *http.Request) (*http.Response, error) {
//	epc, err := c.UGate.H2Transport.FindMux(req.Context(), c)
//	if err != nil {
//		return nil, err
//	}
//
//	r, _, err := c.rt(epc, req)
//	// TODO: grpc RoundTripper doesn't wait for headers !
//	return r, err
//}

// rt is the low level rountdrip implementation, after finding a connection to one endpoint
//func (c *MeshCluster) rt(epc *EndpointCon, req *http.Request) (*http.Response, *EndpointCon, error) {
//	var resp *http.Response
//	var rterr, err error
//
//	c.AddToken(c.UGate.Auth, req, "https://"+c.Addr)
//
//	for i := 0; i < 3; i++ {
//
//		// Find a channel - LB would go here if multiple addresses and sockets
//		if epc == nil || epc.RoundTripper == nil {
//			epc, err = c.UGate.H2Transport.FindMux(req.Context(), c)
//			if err != nil {
//				return nil, nil, err
//			}
//		}
//
//		// IMPORTANT: some servers will not return the headers until the first byte of the response is sent, which
//		// may not happen until request bytes have been sent.
//		// For CONNECT, we will require that the server is flushing the headers as soon as the request is received,
//		// to emulate the connection semantics - at least initially.
//		// For POST and other methods - we can't assume this. That means read() on the conn will need to be blocked
//		// and wait for the Header frame to be received, and any metadata too.
//		resp, rterr = epc.RoundTripper.RoundTrip(req)
//		if Debug {
//			log.Println("RoundTrip", req, resp, rterr)
//		}
//
//		if rterr != nil {
//			// retry on different mux
//			epc.RoundTripper = nil
//			continue
//		}
//
//		return resp, epc, err
//	}
//	return nil, nil, rterr
//}



//// dialHbone implements the tunnel over H2. Returns a net.Conn implementation.
//// TODO(costin): use the hostname, get IP override from x-original-dst header or cookie.
//func (c *MeshCluster) dialHbone(ctx context.Context, req *http.Request) (*EndpointCon, util.Stream, error) {
//  // Find an endpoint and create a H2 connection for it.
//	epc, err := c.UGate.H2Transport.FindMux(ctx, c)
//	if err != nil {
//		return nil, nil, err
//	}
//
//	if epc.Endpoint.Labels["http_proxy"] != "" {
//		// TODO: only POST mode supported right now, address not from label.
//
//		hostPort := epc.Endpoint.HboneAddress()
//
//		// Tunnel mode, untrusted proxy authentication.
//		req, _ := http.NewRequestWithContext(ctx, "POST", "https://"+hostPort, nil)
//
//		err = c.AddToken(c.UGate.Auth, req, "https://"+hostPort)
//		if err != nil {
//			return nil, nil, err
//		}
//
//		req.Header.Add("x-service", c.Addr)
//		req.Header.Add("x-tun", epc.Endpoint.Address)
//
//		res, err := epc.RoundTripper.RoundTrip(req)
//		if err != nil {
//			epc.RoundTripper = nil
//			return nil, nil, err
//		}
//
//		nc := res.Body.(net.Conn)
//		// Do the mTLS handshake for the tunneled connection
//		// SNI is based on the service name - or the SNI override.
//		sni := c.SNI
//		if sni == "" {
//			sni, _, _ = net.SplitHostPort(c.Addr)
//		}
//
//		// Create a TLS connection to the endpoint.
//		// Using the sni of the service (frontend), but check the FQDN hostname identity
//		tlsClientConfig := c.UGate.Auth.TLSClientConf(&epc.Cluster.Dest, sni, epc.Endpoint.Hostname)
//		tlsTun := tls.Client(nc, tlsClientConfig)
//		err = util.HandshakeTimeout(tlsTun, c.UGate.HandsahakeTimeout, nil)
//		if err != nil {
//			return nil, nil, err
//		}
//		return epc, util.NewStreamConn(tlsTun), err
//
//		// TLS wrapper will be added on response.
//		//return epc, res.Body.(net.Conn), err //
//		// &HTTPConn{R: res.Body, W: o, Conn: epc.TLSConn, Req: req, Res: res,
//		//	MeshCluster: c}, err
//	}
//
//	if req == nil {
//		req, _ = http.NewRequestWithContext(ctx, "CONNECT", "https://"+epc.Endpoint.Address, nil)
//	}
//
//	req.Header.Add("x-service", c.Addr)
//
//	res, _, err := c.rt(epc, req)
//	if err != nil {
//		return nil, nil, err
//	}
//
//	nc := res.Body.(util.Stream)
//	// TODO: return nc directly instead of HTTPConn
//	return epc, nc, err
//}
