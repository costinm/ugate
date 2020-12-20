package ugate

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http/httptrace"
	"strconv"
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
func NewForwarder(gw *UGate, cfg *ListenerConf) (*portListener,error) {
	ll := &portListener{
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

	laddr := listener.Addr().String()
	_, port, _ := net.SplitHostPort(laddr)
	portN, _ := strconv.Atoi(port)

	ll.cfg.Port = portN
	ll.Listener = listener

	ll.GW.Listeners[portN] = ll

	go ll.serve()
	return ll, nil
}

// A portListener is similar with an Envoy portListener.
// It can be created by a Gateway or Sidecar resource in istio, as well as from in Service and for out capture
//
// For mesh, it is also auto-created for each device/endpoint/node for accepting messages and in connections.
//
type portListener struct {
	port int32

	listeners []*ListenerConf
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
func (pl *portListener) FindConf(nc net.Conn, prefix []byte) *ListenerConf {
	return nil
}


func (ll *portListener) Close() error {
	ll.Listener.Close()
	delete(ll.GW.Listeners, ll.cfg.Port)
	return nil
}

func (ll portListener) Accept() (net.Conn, error) {
	return ll.Listener.Accept()
}

func (ll portListener) Addr() (net.Addr) {
	return ll.Listener.Addr()
}

// For -R, runs on the remote ssh server to accept connections and forward back to client, which in turn
// will forward to a port/app.
// Blocking.
func (ll portListener) serve() {
	log.Println("Gateway: open on ", ll.cfg.Local, ll.cfg.Remote, ll.cfg.Protocol)
	for {
		remoteConn, err := ll.Listener.Accept()
		VarzAccepted.Add(1)
		if ne, ok := err.(net.Error); ok {
			VarzAcceptErr.Add(1)
			if ne.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		if err != nil {
			return
		}
		go ll.handleAcceptedConn(remoteConn)
	}
}


func (ll *portListener) onDone(rc *AcceptedConn) {
	if rc.Stats.ReadErr != nil {
		VarzSErrRead.Add(1)
	}
	if rc.Stats.WriteErr != nil {
		VarzSErrWrite.Add(1)
	}
	if rc.Stats.ProxyReadErr != nil {
		VarzCErrRead.Add(1)
	}
	if rc.Stats.ProxyWriteErr != nil {
		VarzCErrWrite.Add(1)
	}
	log.Printf("A: %d src=%s://%v dst=%s rcv=%d/%d snd=%d/%d la=%v ra=%v op=%v",
		rc.Stats.StreamId,
		rc.Stats.Type, rc.RemoteAddr(),
		rc.Target,
		rc.Stats.ReadPackets, rc.Stats.ReadBytes,
		rc.Stats.WritePackets, rc.Stats.WriteBytes,
		time.Since(rc.Stats.LastWrite),
		time.Since(rc.Stats.LastRead),
		time.Since(rc.Stats.Open))

	rc.Close()

	bufferedConPool.Put(rc)
}

func (ll *portListener) handleAcceptedConn(rc net.Conn) error {
	// c is the local or 'client' connection in this case.
	// 'remote' is the configured destination.

	//c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

	//ra := rc.RemoteAddr().(*net.TCPAddr)

	// 1. Try to match a Listener based on remote addr/port and local addr/port

	// 2. If we don't have enough info, sniff.
	// Client first protocols can't support multiplexing, must have single config

	// TODO: poll the readers
	br := GetConn(rc)
	defer ll.onDone(br)

	// Attempt to determine the ListenerConf and target
	// Ingress mode, forward to an IP
	cfg := ll.cfg

	if cfg.Remote != "" {
		br.Target = cfg.Remote
	} else if cfg.Endpoint != nil {
	} else if cfg.Protocol == "socks5" {
		ll.GW.serveSOCKSConn(br)
	} else if cfg.Protocol == "sni" {
		ll.GW.serveConnSni(br)
	} else {
		err := ll.sniff(br)
		if err != nil {
			return err
		}
	}

	// sets clientEventContextKey - if ctx is used for a round trip, will
	// set all data.
	// Will also make sure DNSStart, Connect, etc are set (if we want to)
	ctx := httptrace.WithClientTrace(context.Background(), &br.Stats.ClientTrace)

	// SSH or in-process connectors
	if cfg.Endpoint != nil {
		cfg.Endpoint.DialContext(ctx, "tcp", br.Target)
		return nil
	}

	nc, err := ll.GW.Dialer.DialContext(ctx, "tcp", br.Target)
	if br.postDial != nil {
		br.postDial(nc, err)
	}
	if err != nil {
		log.Println("Failed to connect ", cfg.Remote, err)
		return err
	}

	err = br.Proxy(nc)

	// The dialed connection has stats, so does the accept connection.

	return err
}

func (ll *portListener) sniff(br *AcceptedConn) error {
		br.Sniff()
		var proto string

		off := 0
		for {
			n, err := br.Read(br.buf[off:])
			if err != nil {
				return err
			}
			off += n
			if off >= 2 {
				b0 := br.buf[0]
				b1 := br.buf[1]
				if b0 == 5 {
					proto = "socks5"
					break;
				}
				// TLS or SNI - based on the hostname we may terminate locally !
				if b0 == 22 && b1 == 3 {
					// 22 03 01..03
					proto = "sni"
					break
				}
				// TODO: CONNECT, WS else try HTTP/1.1 or HTTP/2 or gRPC
			}
			if off >= 7 {
				if bytes.Equal(br.buf[0:7], []byte("CONNECT")) {
					proto = "ws"
					break
				}
			}
			if off >= len(h2ClientPreface) {
				if bytes.Equal(br.buf[0:len(h2ClientPreface)], h2ClientPreface) {
					proto = "h2"
					break
				}
			}
		}

	// All bytes in the buffer will be Read again
	br.Reset(0)

	switch proto {
	case "socks5":
		ll.GW.serveSOCKSConn(br)
	case "sni":
		ll.GW.serveConnSni(br)
	}

	return nil
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
