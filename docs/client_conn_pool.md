package goh2


// h2 library uses ClientConnPool interface to get http2.ClientConn objects.
// It is tempting to use it as a way to do mesh routing and LB - but my attempt
// didn't look clean enough, so removed.

//func (t *H2Transport) MarkDead(h2c *http2.ClientConn) {
//	t.m.Lock()
//	dmn := t.conns[h2c]
//	if dmn != nil {
//		dmn.RoundTripper = nil
//		log.Println("Dead", dmn.ID, h2c)
//	}
//	t.m.Unlock()
//}


//// GetClientConn returns H2 multiplexed client connection for connecting to a mesh host.
////
//// Part of x.net.http2.ClientConnPool interface.
//// addr is a host:port, based on the URL host.
//// The result implements RoundTrip interface.
//func (t *H2Transport) GetClientConn(ctx context.Context, prot string, addr string) (*http2.ClientConn, error) {
//	// The h2 H2Transport has support for dialing TLS, with the std handshake.
//	// It is possible to replace H2Transport.DialContext, used in clientConnPool
//	// which tracks active connections. Or specify a custom conn pool.
//
//	// addr is either based on req.Host or the resolved IP, in which case Host must be used for TLS verification.
//	host, _, _ := net.SplitHostPort(addr)
//
//	nid := t.ug.Host2ID(addr)
//	// TODO: if mesh node, don't attempt to dial directly
//	dmn := t.ug.GetDest(nid)
//	if dmn != nil {
//		rt := dmn.RoundTripper
//		if rt != nil {
//			if rtc, ok := rt.(*http2.ClientConn); ok {
//				return rtc, nil
//			}
//		}
//
//		// TODO: if we don't have addr, use discovery
//
//		// TODO: if discovery doesn't return an address, use upsteram gate.
//		if dmn.Addr == "" {
//			return nil, NotFound
//		}
//		// Real address -
//		addr = dmn.Addr
//	}
//	var tc nio.Stream
//	var err error
//	// TODO: use local announces
//	// TODO: use VPN server for all or for mesh
//
//	rc, err := t.ug.DialContext(ctx, "tcp", addr)
//	if err != nil {
//		return nil, err
//	}
//
//	// Separate timeout for handshake and connection - ctx is used for the entire connection.
//	to := t.ug.HandsahakeTimeout
//	if to == 0 {
//		to = 5 * time.Second
//	}
//	ctx1, cf := context.WithTimeout(ctx, to)
//	defer cf()
//
//	if prot == "http" {
//		tc = nio.GetStream(rc, rc)
//	} else {
//		tlsc, err := nio.NewTLSConnOut(ctx1, rc, t.ug,
//			host, []string{"h2"})
//		if err != nil {
//			return nil, err
//		}
//		tc = nio.NewStreamConn(tlsc)
//	}
//
//	// TODO: reuse connection or use egress server
//	// TODO: track it by addr
//
//	// This is using the native stack http2.H2Transport - implements RoundTripper
//	cc, err := t.h2t.NewClientConn(tc)
//
//	if dmn != nil {
//		// Forward connection ok too.
//		dmn.RoundTripper = cc
//	}
//	return cc, err
//}
//var NotFound = errors.New("not found")

