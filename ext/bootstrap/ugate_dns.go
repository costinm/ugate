// +build !MIN

package bootstrap

import (
	"net"

	"github.com/costinm/ugate/dns"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		dnss, _ := dns.NewDmDns(5223)
		//GW. = dnss
		net.DefaultResolver.PreferGo = true
		net.DefaultResolver.Dial = dns.DNSDialer(5223)

		ug.DNS = dnss
		return func(ug *ugatesvc.UGate) {
			go dnss.Serve()
		}
	})
}
