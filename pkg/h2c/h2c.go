package h2c

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// At the cost of a x/net dependency, add h2c support (pre 1.24)
//
// The x/net also includes websocket, webdav, quic
//
// The server will listen on a port and manage a mux.
//

// Can't do h2c using the std client - need custom code.
type H2C struct {
	http2.Transport
}

func (h *H2C) Provision(ctx context.Context) error {
	h.AllowHTTP = true
	h.DialTLSContext = func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}

	h.ReadIdleTimeout = 10000 * time.Second
	h.StrictMaxConcurrentStreams = false

	return nil
}

// HTTP2 based transport, using x/net/http2 library directly (instead of standard library).
// curl localhost:9080/debug/vars --http2-prior-knowledge
type H2CD struct {
	// Extends server with the address.
	// Probably should add each setting and copy, but simpler.
	http2.Server

	//MaxHandlers:                  0,
	//MaxConcurrentStreams:         0,
	//MaxDecoderHeaderTableSize:    0,
	//MaxEncoderHeaderTableSize:    0,
	//MaxReadFrameSize:             0,
	//PermitProhibitedCipherSuites: false,
	//IdleTimeout:                  0,
	//MaxUploadBufferPerConnection: 0,
	//MaxUploadBufferPerStream:     0,
	//NewWriteScheduler:            nil,
	//CountError:                   nil,

	Address string `json:"address"`
	Mux     *http.ServeMux
}

func (h *H2CD) Provision(ctx context.Context) error {
	if h.Address == "" {
		h.Address = ":15082"
	}
	if h.Mux == nil {
		h.Mux = http.NewServeMux()
	}
	return nil
}
func (h *H2CD) Start() error {

	// Also start a H2 server - it increases the size from 6.1 to 6.8M, but it seems
	// worth it at this point. May optimize later...
	// It allows the server to run behind an Istio/K8S gateawy or in cloudrun.

	// implements the H2CD protocol - detects requests with PRI and proto HTTP/2.0 and Upgrade - and calls
	// ServeConn.
	//
	// Otherwise, HTTP/1.1 is used.
	h2ch := h2c.NewHandler(h.Mux, &h.Server)

	//h2s.ServeConn(nil, &http2.ServeConnOpts{
	//	Context:          nil,
	//	BaseConfig:       nil,
	//	Handler:          nil,
	//	UpgradeRequest:   nil,
	//	Settings:         nil,
	//	SawClientPreface: false,
	//})

	// TODO: add 	if hb.TCPUserTimeout != 0 {
	//		// only for TCPConn - if this is used for tls no effect
	//		syscall.SetTCPUserTimeout(conn, hb.TCPUserTimeout)
	//	}
	l, err := net.Listen("tcp", h.Address)
	if err != nil {
		return err
	}
	//if ma.HandlerWrapper != nil {
	//	h = ma.HandlerWrapper(h, ll.Address)
	//}

	go http.Serve(l, h2ch)

	return nil
}
