package ugate

import (
	"crypto/tls"
	"expvar"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

var (
	streamIds = int32(0)
)


var (
	VarzAccepted = expvar.NewInt("ugate.accepted")
	VarzAcceptErr = expvar.NewInt("ugate.acceptErr")

	// Proxy behavior
	VarzReadFrom = expvar.NewInt("ugate.sReadFrom")
	VarzReadFromC = expvar.NewInt("ugate.cReadFrom")

	VarzSErrRead = expvar.NewInt("ugate.sErrRead")
	VarzSErrWrite = expvar.NewInt("ugate.sErrWrite")
	VarzCErrRead = expvar.NewInt("ugate.cErrRead")
	VarzCErrWrite = expvar.NewInt("ugate.cErrWrite")

	VarzMaxRead = expvar.NewInt("ugate.maxRead")
)

func (s *Stats) Reset() {
	s.Open = time.Now()
	s.LastRead = time.Time{}
	s.LastWrite = time.Time{}

	s.ReadBytes = 0
	s.WriteBytes = 0
	s.ReadPackets = 0
	s.WritePackets = 0

	s.ReadErr = nil
	s.WriteErr = nil
	s.Type = ""

	s.StreamId = atomic.AddInt32(&streamIds, 1)
}

type UGate struct {
	Listeners map[string]*PortListener

	Dialer ContextDialer

	// Configurations, keyed by port.
	Conf map[int][]*ListenerConf

	tlsConfig tls.Config
}

func NewGate(d ContextDialer) *UGate {
	return &UGate{
		Listeners: map[string]*PortListener{},
		Dialer: d,
	}
}

// WIP: add a TCP Gateway spec and the matching routes.
// Gateway is keyed by: [address, port] and can dispatch by hostname
//
// It is expected this is already filtered and matched by the control plane,
// the gateway gets the raw config it needs to apply.
//
// For HTTPRoute: ugate does not process http except hostname, instead a TCPRoute to a
// HTTP gateway (egress GW) can be used.
func (ug *UGate) AddGateway(cfg *Gateway, routes []*TCPRoute) (error) {
	// TODO: break down the gateway by Address and Listener.
	// Each combination results in a ListenerConf

	//l, err := NewListener(ug, cfg)
	//if err != nil {
	//	return nil, nil, err
	//}
	//return l, l.Listener.Addr(), nil
	return nil
}

func (ug *UGate) Add(cfg *ListenerConf) (*PortListener, net.Addr, error) {
	l, err := NewListener(ug, cfg)
	if err != nil {
		return nil, nil, err
	}
	ug.Listeners[l.cfg.Local] = l
	return l, l.Listener.Addr(), nil
}

// TODO
func (ug *UGate) Remove(cfg *ListenerConf) {

}

func (ug *UGate) Close() error {
	var err error
	for _, p := range ug.Listeners {
		e := p.Close()
		if e != nil {
			err = err
		}
		delete(ug.Listeners, p.cfg.Local)
	}
	return err
}

// Setup default ports, using base port.
// For Istio, should be 15000
func (ug *UGate) DefaultPorts(base int) error {
	// Not on localhost - redirect changes the port
	ug.Add(&ListenerConf{
		Port: base + 1,
		Protocol: "iptables",
	})
	ug.Add(&ListenerConf{
		Port: base + 6,
		Protocol: "iptables-in",
	})
	// Normally should be 443 for gateways
	ug.Add(&ListenerConf{
		Port: base + 3,
		Protocol: "sni",
	})
	ug.Add(&ListenerConf{
		Local: fmt.Sprintf("127.0.0.1:%d", base + 9),
		Protocol: "socks5",
	})
	ug.Add(&ListenerConf{
		Port: base + 4,
		Protocol: "mux",
	})

	return nil
}
