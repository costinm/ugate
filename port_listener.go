package ugate

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RemoteListener accepts connections on a port, and proxies over the mesh.
//
// If the address is 0.0.0.0, it is similar with -R in ssh, i.e. the node is the
// ingress. It is also similar with TURN.
//
// If the address is 127.0.0.1 it can be used to create a proxy for a local app
// as an alternative to SOCKS or iptables.

// Similar with go-ipfs/p2p

// K8S:
// API_SERVER/api/v1/namespaces/%s/pods/%s/portforward
// Forwards a local port to the pod, using SPDY or Websocket.


// Docs and other options:
//https://blog.ston3o.me/how-to-expose-local-server-behind-firewall/
// - OpenVPN - easy to setup docker container
// - upnpc
// - tor
// ngrok - free 40 con/min
// pagekite - py, $3/month
// bleenco/localtunnel (go)
// localtunnel/localtunnel (js)
// yaler - commercial
// inlets / rancher remote dialer

// socks bind standard - not commonly implemented

// ssh -R remote_server_ip:12345:localhost:12345
// - multiplexed over ssh TCP con, flow control per socket

/*
			byte      SSH_MSG_CHANNEL_OPEN
      string    "forwarded-tcpip"
      uint32    sender channel

			uint32    initial window size
      uint32    maximum packet size

			string    address that was connected
      uint32    port that was connected

			string    originator IP address
      uint32    originator port
*/

// concourse TSA - uses ssh, default 2222
// 'beacon' is effectively using ssh command to forward ports
// "golang.org/x/crypto/ssh"
//https://github.com/concourse/tsa/blob/master/tsacmd/server.go

// Original implementation attempted to use http(2).
// The main problem with H2 client connections is that we lack ability to flush() on the input
// stream. This is a problem for the http interface in go, and unfortunately I'm not aware of
// any good solution.
// 1. We can use just the response stream, creating a new connection to send response.
// The new connection may go to a different replica - so some forwarding on server side
// may be needed. Ok with a single server, or if the server can be pinned (cookie, etc)
// 2. We can use the low level h2 stack, like grpc http2_client.

// Rancher 'Reverse Tunneling Dialer' and 'inlets':
// - use websocket - no multiplexing.
// - binary messages, using websocket frames

// TODO: emulate SSHClientConn protocol over H3 ( H2 connections framed )
// TODO: send a message to request client to open a reverse TCP channel for each accepted connection
var (
	// ClientPreface is the string that must be sent by new
	// connections from clients.
	h2ClientPreface = []byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")
)

// Creates a raw (port) TCP listener. Accepts connections
// on a local port, forwards to a remote destination.
func NewListener(gw *UGate, cfg *ListenerConf) (*PortListener,error) {
	ll := &PortListener{
		cfg:  cfg,
		Gate: gw,
	}

	if cfg.Host == "" {
		cfg.Host = fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	} else {
		_, port, err := net.SplitHostPort(cfg.Host)
		if err != nil {
			return nil, err
		}
		cfg.Port, err = strconv.Atoi(port)
		if err != nil {
			return nil, err
		}
	}

	if cfg.Listener == nil {
		if strings.HasPrefix(cfg.Host, "/") ||
				strings.HasPrefix(cfg.Host, "@"){
			us, err := net.ListenUnix("unix",
				&net.UnixAddr{
					Name: cfg.Host,
					Net: "unix",
				})
			if err != nil {
				return nil, err
			}
			cfg.Listener = us
		} else {
			// Not supported: RFC: address "" means all families, 0.0.0.0 IP4, :: IP6, localhost IP4/6, etc
			listener, err := net.Listen("tcp", ll.cfg.Host)
			if err != nil {
				host, _, _ := net.SplitHostPort(ll.cfg.Host)
				ll.cfg.Host = host + ":0"
				listener, err = net.Listen("tcp", ll.cfg.Host)
				if err != nil {
					log.Println("failed-to-listen", err)
					return nil, err
				}
			}
			cfg.Listener = listener
		}
	}

	laddr := cfg.Listener.Addr().String()
	_, port, _ := net.SplitHostPort(laddr)
	portN, _ := strconv.Atoi(port)

	ll.cfg.Port = portN
	ll.Listener = cfg.Listener

	go ll.serve()
	return ll, nil
}

// A PortListener is similar with an Envoy PortListener.
// It can be created by a Gateway or Sidecar resource in istio, as well as from in Service and for out capture
//
// For mesh, it is also auto-created for each device/endpoint/node for accepting messages and in connections.
//
type PortListener struct {
	port int32

	cfg *ListenerConf
	// Destination:
	// - sshConn if set -
	// - Remote
	// - vpn (for outbound) ?
	// - dmesh ingress gateway

	// Real listener for the port
	Listener net.Listener

	Gate *UGate
}

// FindConf handles routing of the incoming connection to the right Listener object.
//
func (pl *PortListener) FindConf(nc net.Conn, prefix []byte) *ListenerConf {
	return nil
}


func (pl *PortListener) Close() error {
	pl.Listener.Close()
	return nil
}

func (pl PortListener) Accept() (net.Conn, error) {
	return pl.Listener.Accept()
}

func (pl PortListener) Addr() (net.Addr) {
	return pl.Listener.Addr()
}

// For -R, runs on the remote ssh server to accept connections and forward back to client, which in turn
// will forward to a port/app.
// Blocking.
func (pl *PortListener) serve() {
	log.Println("Gateway: open on ", pl.cfg.Host, pl.cfg.Remote, pl.cfg.Protocol)
	for {
		remoteConn, err := pl.Listener.Accept()
		VarzAccepted.Add(1)
		if ne, ok := err.(net.Error); ok {
			VarzAcceptErr.Add(1)
			if ne.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		if err != nil {
			log.Println("Accept error, closing listener ", pl.cfg, err)
			return
		}
		go pl.Gate.handleAcceptedConn(pl, remoteConn)
	}
}

// Dial the target and proxy to it.
func (ug *UGate) dialOut(bconn MetaConn, cfg *ListenerConf) error {
	// sets clientEventContextKey - if ctx is used for a round trip, will
	// set all data.
	// Will also make sure DNSStart, Connect, etc are set (if we want to)
	//ctx, cancel := context.WithCancel(bconn.Meta().Request.Context())
	//ctx := httptrace.WithClientTrace(bconn.Meta().Request.Context(),
	//	&bconn.Meta().ClientTrace)
	str := bconn.Meta()
	ctx := str.Request.Context()
	r := str.Request
	//ctx = context.WithValue(ctx, "ugate.conn", bconn)
	var nc net.Conn
	var err error

	// Dial out via an existing 'reverse h2' connection
	rt := ug.h2Handler.H2R[str.Request.Host]
	if rt != nil {
		// We have an active reverse H2 - use it
		// TODO: move to ProxyHTTP
		r1, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
		CopyHeaders(r1.Header, r.Header)
		r1.URL.Scheme = "https"
		// RT client - forward the request.
		res, err := rt.RoundTrip(r1)
		if err != nil {
			log.Println("Failed to do H2R", err)
			return err
		}
		CopyHeaders(str.ResponseHeader, res.Header)
		str.WriteHeader(res.StatusCode)
		str.Flush()
		str.CopyBuffered(str, res.Body, true)
		return nil
	}

	// SSH or in-process connectors
	if cfg.Dialer != nil {
		nc, err = cfg.Dialer.DialContext(ctx, "tcp", bconn.Meta().Request.Host)
	} else {
		nc, err = ug.Dialer.DialContext(ctx, "tcp", bconn.Meta().Request.Host)
	}

	if pc, ok := bconn.(ProxyConn); ok {
		pc.PostDial(nc, err)
	}

	if err != nil {
		log.Println("Failed to connect ", bconn.Meta().Request.Host, err)
		return err
	}
	defer nc.Close()

	/////////////log.Println("Connected RA=", nc.RemoteAddr(), nc.LocalAddr())

	if cfg.RemoteTLS != nil {
		//var clientCon *RawConn
		//if clCon, ok := nc.(*RawConn); ok {
		//	clientCon = clCon
		//} else {
		//	clientCon = GetConn(nc)
		//	clientCon.Meta.Accepted = false
		//}
		lconn, err := NewTLSConnOut(ctx, nc, cfg.RemoteTLS, "", nil)
		if err != nil {
			return err
		}
		nc = lconn
	}

	return bconn.(ProxyConn).ProxyTo(nc)
}


// port capture is the plain reverse proxy mode: it listens to a port and forwards.
//
// Clients will use "localhost:port" for TCP or UDP proxy, and http will use some DNS
// resolver override to map hostname to localhost.
// The config is static (mesh config) or it can be dynamic (http admin interface or mesh control)

// Start a port capture or forwarding.
// listenPort: port on local host, or :0. May include 127.0.0.1 or 0.0.0.0 or specific interface.
// host: destination. Any connection on listenPort will result on a TCP stream to the destination.
//       May be a chain of DMesh nodes, with an IP:port at the end.
