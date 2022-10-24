package main

import (
	"log"
	_ "net/http/pprof"

	"github.com/costinm/ugate/pkg/ugatesvc"

	_ "github.com/costinm/ugate/ugated"
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
// - DNS and DNS capture (if root)
// - control plane using webpush messaging
// - webRTC and H2 for mesh communication
// - convert from H2/H1, based on local port config.
//
// Extras:
// - SOCKS and PROXY
//
// This does not include lwIP, which is now only used with AndroidVPN in
// JNI mode.
func main() {
	// Load configs from the current dir and var/lib/dmesh, or env variables
	// Writes to current dir.
	config := ugatesvc.NewConf("./", "./var/lib/dmesh")

	_, err := ugatesvc.Run(config, nil)
	if err != nil {
		log.Fatal(err)
	}
	select {}
}
