package ugate

import (
	"context"
	"fmt"
	"log"
	"net"
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
		cfg: cfg,
		GW: gw,
	}

	if cfg.Local == "" {
		cfg.Local = fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	} else {
		_, port, err := net.SplitHostPort(cfg.Local)
		if err != nil {
			return nil, err
		}
		cfg.Port, err = strconv.Atoi(port)
		if err != nil {
			return nil, err
		}
	}

	if cfg.Listener == nil {
		if strings.HasPrefix(cfg.Local, "/") ||
				strings.HasPrefix(cfg.Local, "@"){
			us, err := net.ListenUnix("unix",
				&net.UnixAddr{
					Name: cfg.Local,
					Net: "unix",
				})
			if err != nil {
				return nil, err
			}
			cfg.Listener = us
		} else {
			// Not supported: RFC: address "" means all families, 0.0.0.0 IP4, :: IP6, localhost IP4/6, etc
			listener, err := net.Listen("tcp", ll.cfg.Local)
			if err != nil {
				host, _, _ := net.SplitHostPort(ll.cfg.Local)
				ll.cfg.Local = host + ":0"
				listener, err = net.Listen("tcp", ll.cfg.Local)
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

	GW *UGate
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
func (pl PortListener) serve() {
	log.Println("Gateway: open on ", pl.cfg.Local, pl.cfg.Remote, pl.cfg.Protocol)
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
		go pl.handleAcceptedConn(remoteConn)
	}
}

// Dial the target and proxy to it.
func (pl *PortListener) dialOut(ctx context.Context, bconn MetaConn, cfg *ListenerConf) error {
	var nc net.Conn
	var err error
	// SSH or in-process connectors
	if cfg.Dialer != nil {
		nc, err = cfg.Dialer.DialContext(ctx, "tcp", bconn.Meta().Target)
	} else {
		nc, err = pl.GW.Dialer.DialContext(ctx, "tcp", bconn.Meta().Target)
	}
	//if err != nil {
	//	return err
	//}

	bconn.PostDial(nc, err)
	if err != nil {
		log.Println("Failed to connect ", cfg.Remote, err)
		return err
	}

	if cfg.RemoteTLS != nil {
		//var clientCon *RawConn
		//if clCon, ok := nc.(*RawConn); ok {
		//	clientCon = clCon
		//} else {
		//	clientCon = GetConn(nc)
		//	clientCon.Stats.Accepted = false
		//}
		lconn, err := NewTLSConnOut(ctx, nc, cfg.RemoteTLS, "")
		if err != nil {
			return err
		}
		nc = lconn
	}

	return bconn.Proxy(nc)

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
