package main

import (
	"fmt"
	"log"
	"net"
	_ "net/http/pprof"


	"github.com/costinm/ugate"
	"github.com/costinm/ugate/dns"
	"github.com/costinm/ugate/pkg/http_proxy"
	"github.com/costinm/ugate/pkg/socks"
	"github.com/costinm/ugate/pkg/udp"
	"github.com/costinm/ugate/pkg/ugatesvc"

	_ "github.com/costinm/ugate/ext/bootstrap"
	_ "github.com/costinm/ugate/ext/bootstrapx"
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
	Run(config, nil)
	select {}
}

func Run(config ugate.ConfStore, g *ugate.GateCfg) (*ugatesvc.UGate, error){
	// Start a Gate. Basic H2 and H2R services enabled.
	ug := ugatesvc.New(config, nil, g)

	sf := []ugatesvc.StartFunc{}
	if ugatesvc.InitHooks != nil {
		for _, h := range ugatesvc.InitHooks {
			s := h(ug)
			if s != nil {
				sf = append(sf, s)
			}
		}
	}

	// Init DNS capture and server
	dnss, _ := dns.NewDmDns(5223)
	//GW. = dnss
	net.DefaultResolver.PreferGo = true
	net.DefaultResolver.Dial = dns.DNSDialer(5223)

	ug.DNS = dnss
	// UDP Gate is used with TProxy and lwIP.
	udp.New(ug)

	hproxy := http_proxy.NewHTTPProxy(ug)
	hproxy.HttpProxyCapture(fmt.Sprintf("127.0.0.1:%d", ug.Config.BasePort+ugate.PORT_HTTP_PROXY))

	socks.New(ug)

	go dnss.Serve()

	for _, h := range sf {
		go h(ug)
	}
	ug.Start()
	log.Println("Started: ", ug.Auth.ID)
	return ug, nil
}
