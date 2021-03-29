package main

import (
	"net"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

// Minimal uGate - not using any optional package.
// Used to determine the 'base' size and cost of various options.
//
// Excluded:
// - WebRTC
// - iptables or TUN
// - DNS
//
// Defaults to port 12000, with a Iperf3 forward on 12011

func main() {
	config := ugatesvc.NewConf(".", "./var/lib/dmesh")
	cfg := &ugate.GateCfg{
		BasePort: 12000,
		Domain: "h.webinf.info",
	}
	// Start a Gate. Basic H2 and H2R services enabled.
	ug := ugatesvc.NewGate(&net.Dialer{}, nil, cfg, config)

	// direct TCP connect to local iperf3 and fortio (or HTTP on default port)
	ug.Add(&ugate.Listener{
		Address: ":12011",
		ForwardTo: "localhost:5201",
	})

	select {}
}
