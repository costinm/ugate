package sni

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/costinm/ssh-mesh/nio"
	"github.com/costinm/ugate"
)

type SNIHandler struct {
	UGate *ugate.UGate
	Dialer net.Dialer
}

// HandleSNIConn implements SNI based routing. This can be used for compat
// with Istio. Was original method to tunnel for serverless.
//
// This can be used for a legacy CNI to UGate bridge. The old Istio client expects an mTLS connection
// to the other end - the UGate proxy is untrusted.
func (snih *SNIHandler) HandleConn(conn net.Conn) error {
	hb := snih.UGate

	s := nio.NewBufferReader(conn)
	defer conn.Close()
	defer s.Buffer.Recycle()

	cn, sni, err := nio.SniffClientHello(s)
	if err != nil {
		return err
	}
	if cn == nil {
		// First bytes are not TLS
		return errors.New("Not TLS")
	}

	// At this point we have a SNI service name. Need to convert it to a real service
	// name, RoundTripStart and proxy.

	addr := sni + ":443"
	// Based on SNI, make a hbone request, using JWT auth.
	if strings.HasPrefix(sni, "outbound_.") {
		// Current Istio SNI looks like:
		//
		// outbound_.9090_._.prometheus-1-prometheus.mon.svc.cluster.local
		// We need to map it to a cloudrun external address, add token based on the audience, and
		// make the call using the tunnel.
		//
		// Also supports the 'natural' form and egress

		//
		//
		parts := strings.SplitN(sni, ".", 4)
		remoteService := parts[3]
		// TODO: extract 'version' from URL, convert it to cloudrun revision ?
		addr = net.JoinHostPort(remoteService, parts[1])
	}

	ctx := context.Background()
	nc, err := hb.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	err = nio.Proxy(nc, s, conn, addr)
	if err != nil {
		return err
	}
	return nil
}
