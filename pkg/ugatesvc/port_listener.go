package ugatesvc

import (
	"log"
	"net"
	"strings"
	"time"

	"github.com/costinm/ugate"
)

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

// Rancher 'Reverse Tunneling Dialer' and 'inlets':
// - use websocket - no multiplexing.
// - binary messages, using websocket frames

type Listener struct {
	ugate.Listener
}

// Creates a raw (port) TCP listener. Accepts connections
// on a local port, forwards to a remote destination.
func (cfg *Listener) start(gate *UGate) error {
	ll := cfg

	if cfg.Address == "" {
		cfg.Address = ":0"
	}

	if cfg.Address[0] == '-' {
		return nil // virtual listener
	}

	if cfg.NetListener == nil {
		if strings.HasPrefix(cfg.Address, "/") ||
				strings.HasPrefix(cfg.Address, "@") {
			us, err := net.ListenUnix("unix",
				&net.UnixAddr{
					Name: cfg.Address,
					Net:  "unix",
				})
			if err != nil {
				return err
			}
			cfg.NetListener = us
		} else {
			// Not supported: RFC: address "" means all families, 0.0.0.0 IP4, :: IP6, localhost IP4/6, etc
			listener, err := net.Listen("tcp", ll.Address)
			if err != nil {
				host, _, _ := net.SplitHostPort(ll.Address)
				ll.Address = host + ":0"
				listener, err = net.Listen("tcp", ll.Address)
				if err != nil {
					log.Println("failed-to-listen", err)
					return err
				}
			}
			cfg.NetListener = listener
		}

	}

	go ll.serve(gate)
	return nil
}

func (pl *Listener) Close() error {
	pl.NetListener.Close()
	return nil
}

func (pl Listener) Accept() (net.Conn, error) {
	return pl.NetListener.Accept()
}

func (pl Listener) Addr() net.Addr {
	if pl.NetListener == nil {
		return nil
	}
	return pl.NetListener.Addr()
}

// For -R, runs on the remote ssh server to accept connections and forward back to client, which in turn
// will forward to a Port/app.
// Blocking.
func (pl *Listener) serve(gate *UGate) {
	log.Println("Gateway: open on ", pl.Address, pl.ForwardTo, pl.Protocol)
	for {
		remoteConn, err := pl.NetListener.Accept()
		ugate.VarzAccepted.Add(1)
		if ne, ok := err.(net.Error); ok {
			ugate.VarzAcceptErr.Add(1)
			if ne.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		if err != nil {
			log.Println("Accept error, closing listener ", pl, err)
			return
		}
		go gate.handleAcceptedConn(&pl.Listener, remoteConn)
	}
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
