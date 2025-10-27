package http_proxy

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/costinm/meshauth"
	"github.com/costinm/ugate/nio2"
)

// Used for HTTP_PROXY=localhost:port, to intercept outbound traffic using http
// proxy protocol.
//
// It also handles CONNECT and 'transparent' proxy.
//
//

// Android allows using HTTP_PROXY, and is used by browser - more efficient than TUN.

// HttpProxy handles HTTP PROXY and plain text HTTP requests (primarily on port 80)
// for egress side.
//
// A DNAT or explicit proxy are sufficient - no need for TPROXY or REDIRECT, host
// is extracted from request and sessions should use cookies.
type HttpProxy struct {
	gw *meshauth.Mesh

	NetListener net.Listener

	Transport *http.Transport
}

// RoundTripStart listening on the addr, as a HTTP_PROXY
// Handles CONNECT and PROXY requests using the gateway
// for streams.
func (gw *HttpProxy) Start(ctx context.Context) error {
	go http.Serve(gw.NetListener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "CONNECT" {
			gw.handleConnect(w, r)
			return
		}
		// This is a real HTTP proxy
		if r.URL.IsAbs() {
			log.Println("HTTPPRX", r.Method, r.Host, r.RemoteAddr, r.URL)
			gw.captureHttpProxyAbsURL(w, r)
			return
		}
		gw.captureHttpProxyAbsURL(w, r)
	}))
	return nil
}

func ForwardHTTP(c *meshauth.Dest, w http.ResponseWriter, r *http.Request, pathH string) error {

	r.Host = pathH
	r1 := nio2.CreateUpstreamRequest(w, r)

	r1.URL.Scheme = "http"

	// will be used by RoundTrip.
	r1.URL.Host = pathH

	res, err := c.RoundTrip(r1)
	if err != nil {
		return err
	}
	nio2.SendBackResponse(w, r, res, err)
	return nil
}

// Http proxy to a configured HTTP host. Hostname to HTTP address explicitly
// configured. Also hostnmae to file serving.
func (gw *HttpProxy) proxy(w http.ResponseWriter, r *http.Request) bool {
	// TODO: if host is XXXX.m.SUFFIX -> forward to node.

	host, err := gw.gw.Discover(r.Context(), r.Host)
	if err != nil {
		return false
	}
	if len(host.Addr) > 0 {
		log.Println("FWDHTTP: ", r.Method, r.Host, r.RemoteAddr, r.URL)
		ForwardHTTP(host, w, r, host.Addr)
	}
	return true
}

// WIP: HTTP proxy with absolute address, to a QUIC server (or sidecar)`
func (gw *HttpProxy) captureHttpProxyAbsURL(w http.ResponseWriter, r *http.Request) {
	// HTTP proxy mode - uses the QUIC client to connect to the node
	// TODO: redirect via VPN, only root VPN can do plaintext requests

	// parse r.URL, follow the same steps as TCP - if mesh use Client/mtls, if VPN set forward to VPN, else use H2 client

	// r.Host is populated from the absolute URL.
	// Typical headers (curl):
	// User-Agent, Acept, Proxy-Connection:Keep-Alive

	if gw.proxy(w, r) {
		// found the host in clusters - it is an internal/mesh request
		return
	}

	ht := gw.Transport

	resp, err := ht.RoundTrip(r)
	if err != nil {
		log.Println("XXX ", err)
		http.Error(w, err.Error(), 500)
		return
	}
	origBody := resp.Body
	defer origBody.Close()
	nio2.CopyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	log.Println("PHTTP: ", r.URL)

}

// WIP: If method is CONNECT - operate in TCP proxy mode. This can be used to proxy
// a TCP UdpNat to a mesh node, from localhost or from a net node.
// Only used to capture local traffic - should be bound to localhost only, like socks.
// It speaks HTTP/1.1, no QUIC
func (gw *HttpProxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	hij, ok := w.(http.Hijacker)
	if !ok {
		w.WriteHeader(503)
		w.Write([]byte("Error - no hijack support"))
		return
	}

	host := r.URL.Host
	if !strings.Contains(host, ":") {
		host = host + ":443"
	}

	// TODO: second param may contain unprocessed data.
	proxyClient, clientBuffer, e := hij.Hijack()
	if e != nil {
		w.WriteHeader(503)
		w.Write([]byte("Error - no hijack support"))
		return
	}

	//ra := proxyClient.RemoteAddr().(*net.TCPAddr)

	str := nio2.GetStream(proxyClient, proxyClient)
	if clientBuffer.Reader.Size() > 0 {

	}
	//gw.gw.OnStream(str)
	//defer gw.gw.OnStreamDone(str)

	str.Dest = host
	str.Direction = nio2.StreamTypeOut

	nc, err := gw.gw.DialContext(context.Background(), "tcp", str.Dest)

	if err != nil {
		w.WriteHeader(503)
		w.Write([]byte("RoundTripStart error" + err.Error()))
		return
	}
	proxyClient.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))

	nio2.Proxy(nc, str, str, str.Dest)

	//defer nc.Close()
	//
	//str.ProxyTo(nc)
}
