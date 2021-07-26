package main

import (
	"net"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

var initHooks []func(gate *ugatesvc.UGate)

// WIP. TODO:
// - Listens on 15009 as H2C
// - Forwards /hbone/PORT to port
// - Handles /hbone/mtls as mtls and forwards to 8080 as H2C
// - intercepts egress with iptables, forwards to
//   a gate.
//
func main() {
	config := ugatesvc.NewConf(".", "./var/lib/dmesh")
	cfg := &ugate.GateCfg{
		BasePort: 12000,
		Domain: "h.webinf.info",
	}
	// Start a Gate. Basic H2 and H2R services enabled.
	ug := ugatesvc.NewGate(&net.Dialer{}, nil, cfg, config)

	if initHooks != nil {
		for _, h := range initHooks {
			h(ug)
		}
	}

	// direct TCP connect to local iperf3 and fortio (or HTTP on default port)
	ug.Add(&ugate.Listener{
		Address: ":15009",
		ForwardTo: "localhost:5201",
	})

	select {}
}
