package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/costinm/ugate"
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
	config := ugate.NewConf(".")

	auth := ugate.NewAuth(config, "", "h.webinf.info")

	cfg := &ugate.GateCfg{
		BasePort: 15000,
	}

	data, err := ioutil.ReadFile("h2gate.json")
	if err != nil {
		json.Unmarshal(data, cfg)
	}

	// By default, pass through using net.Dialer
	ug := ugate.NewGate(&net.Dialer{}, auth)

	ug.DefaultPorts(cfg.BasePort)

	// direct TCP connect to local iperf3 and fortio (or HTTP on default port)
	ug.Add(&ugate.ListenerConf{
		Port: cfg.BasePort + 101,
		Remote: "localhost:5201",
	})
	ug.Add(&ugate.ListenerConf{
		Port: cfg.BasePort + 108,
		Remote: "localhost:8080",
	})

	ug.Add(&ugate.ListenerConf{
		Port: cfg.BasePort + 102,
		Protocol: "tls",
		TLSConfig: ug.TLSConfig,
		//Remote: "localhost:4444",
		Remote: "localhost:5201",
	})
	ug.Add(&ugate.ListenerConf{
		Port: cfg.BasePort + 103,
		Protocol: "tcp",
		Remote: "localhost:15102", // The TLS server
		RemoteTLS: ug.TLSConfig,
	})
	ug.Add(&ugate.ListenerConf{
		Port: cfg.BasePort + 104,
		Protocol: "tcp",
		Remote: "localhost:15101",
	})

	log.Println("Started UID/GID", os.Getuid(), os.Getegid())
	err = http.ListenAndServe(fmt.Sprintf(":%d", cfg.BasePort + 2), nil)
	if err != nil {
		log.Fatal(err)
	}
}

