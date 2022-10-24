package ugatesvc

import (
	"errors"
	"log"
	"net"
	"strings"
	"time"

	"github.com/costinm/hbone"
	"github.com/costinm/hbone/nio"
	"github.com/costinm/ugate"
)

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

// StartListener and Start a real port listener on a port.
// Virtual listeners can be added to ug.Conf or the mux.
func (ug *UGate) StartListener(ll *hbone.Listener) error {

	err := ug.startPortListener(ll)
	if err != nil {
		return err
	}

	return nil
}

// Creates a raw (port) TCP listener. Accepts connections
// on a local port, forwards to a remote destination.
func (gate *UGate) startPortListener(pl *hbone.Listener) error {
	ll := pl

	if pl.Address == "" {
		pl.Address = ":0"
	}

	if pl.Address[0] == '-' {
		return nil // virtual listener
	}

	if pl.NetListener == nil {
		if strings.HasPrefix(pl.Address, "/") ||
			strings.HasPrefix(pl.Address, "@") {
			us, err := net.ListenUnix("unix",
				&net.UnixAddr{
					Name: pl.Address,
					Net:  "unix",
				})
			if err != nil {
				return err
			}
			pl.NetListener = us
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
			pl.NetListener = listener
		}
	}
	switch pl.Protocol {
	case ugate.ProtoTLS:
		pl.PortHandler = nio.HandlerFunc(gate.handleSNI)
	case ugate.ProtoSNI:
		pl.PortHandler = nio.HandlerFunc(gate.handleSNI)
	case ugate.ProtoBTS:
		pl.PortHandler = nio.HandlerFunc(gate.acceptedHbone)
	case ugate.ProtoBTSC:
		pl.PortHandler = nio.HandlerFunc(gate.acceptedHboneC)
	case ugate.ProtoHTTP:
		pl.PortHandler = nio.HandlerFunc(gate.H2Handler.handleHTTPListener)
	case ugate.ProtoTCPOut:
		if pl.ForwardTo == "" {
			return errors.New("invalid TCPOut, missing ForwardTo")
		}
		pl.PortHandler = nio.HandlerFunc(gate.handleTCPEgress)
	case ugate.ProtoTCPIn:
		if pl.ForwardTo == "" {
			return errors.New("invalid TCPIn, missing ForwardTo")
		}
		pl.PortHandler = nio.HandlerFunc(gate.handleTCPForward)
	default:
		log.Println("Unspecified port, default to forward (in)")
		pl.PortHandler = nio.HandlerFunc(gate.handleTCPForward)
	}

	go serve(ll, gate)
	return nil
}

// For -R, runs on the remote ssh server to accept connections and forward back to client, which in turn
// will forward to a Port/app.
// Blocking.
func serve(pl *hbone.Listener, gate *UGate) {
	log.Println("uGate: listen ", pl.Address, pl.NetListener.Addr(), pl.ForwardTo, pl.Protocol, pl.Handler, pl.PortHandler)
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
		if pl.PortHandler != nil {
			go func() {
				bconn := nio.GetStream(remoteConn, remoteConn)
				bconn.Listener = pl
				bconn.Type = pl.Protocol
				gate.OnStream(bconn)
				defer gate.OnStreamDone(bconn)

				pl.PortHandler.Handle(bconn)
			}()
			return
		}
	}
}
