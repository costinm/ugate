package main

import (
	"encoding/json"
	"log"
	"net"

	_ "net/http/pprof"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/local"
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
	config := ugate.NewConf("./")

	cfg := &ugate.GateCfg{
		BasePort: 15000,
		Domain: "h.webinf.info",
		H2R: map[string]string{
			"c1.webinf.info": "",
		},
	}

	data, err := config.Get("ugate.json")
	if err == nil && data != nil {
		err = json.Unmarshal(data, cfg)
		if err != nil {
			log.Println("Error parsing json ", err, string(data))
		}
	}

	auth := ugate.NewAuth(config, cfg.Name, cfg.Domain)
	// By default, pass through using net.Dialer
	ug := ugate.NewGate(&net.Dialer{}, auth, cfg)

	localgw := local.NewLocal(ug, auth)
	local.ListenUDP(localgw)
	ug.Mux.HandleFunc("/dmesh/ll/if", localgw.HttpGetLLIf)

	log.Println("Started: ", auth.ID)
	select {}
}
