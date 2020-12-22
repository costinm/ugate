package main

import (
	"net"
	"net/http"

	"github.com/costinm/ugate"
)


func main() {
	ug := ugate.NewGate(&net.Dialer{})

	// Using standard net.Dialer, connect to local iperf3
	ug.Add(&ugate.ListenerConf{
		Port: 3001,
		Remote: "localhost:5201",
	})

	// Normally should be 443 for gateways
	ug.Add(&ugate.ListenerConf{
		Port: 15003,
		Protocol: "sni",
	})
	ug.Add(&ugate.ListenerConf{
		Local: "127.0.0.1:15002",
		Protocol: "socks5",
	})
	// Not on localhost - redirect changes the port
	ug.Add(&ugate.ListenerConf{
		Port: 15001,
		Protocol: "iptables",
	})
	ug.Add(&ugate.ListenerConf{
		Port: 15006,
		Protocol: "iptables-in",
	})
	// In-process dialer (ssh, etc)
	//ug.Add(&ugate.ListenerConf{
	//	Port: 3004,
	//	Endpoint: td,
	//})

	http.ListenAndServe(":3010", nil)
}

