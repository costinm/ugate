package main

import (
	"log"
	"net"
	_ "net/http/pprof"

	"github.com/costinm/ugate/pkg/local"
	"github.com/costinm/ugate/pkg/msgs"
	ug "github.com/costinm/ugate/pkg/ugatesvc"
)

// Minimal TCP over H2 Gateway, defaulting to Istio ports and capture behavior.
// There is no envoy - all traffic is upgraded to optional mTLS + H2 and sent to
// a gateway that does additional routing.
//
// This attempts to emulate the 'BTS' design, moving Envoy out of the pod, possibly
// to a shared pool.
//
// - iptables capture
// - option to use mTLS - if the network is secure ( ipsec or equivalent ) no encryption
// - detect TLS and pass it through
// - inbound: extract metadata
// - convert from H2/H1, based on local port config.
//
// Extras:
// - SOCKS and PROXY
//
func main() {
	// Load configs from the current dir and var/lib/dmesh, or env variables
	// Writes to current dir.
	config := ug.NewConf("./", "./var/lib/dmesh")
	// Start a Gate. Basic H2 and H2R services enabled.
	ug := ug.NewGate(&net.Dialer{}, nil, nil, config)

	msgs.DefaultMux.Auth = ug.Auth

	// Discover local nodes using multicast UDP
	localgw := local.NewLocal(ug, ug.Auth)
	local.ListenUDP(localgw)
	ug.Mux.HandleFunc("/dmesh/ll/if", localgw.HttpGetLLIf)

	log.Println("Started: ", ug.Auth.ID)
	select {}
}
