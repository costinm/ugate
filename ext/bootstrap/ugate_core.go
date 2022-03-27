// +build !MIN

package bootstrap

import (
	"fmt"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/http_proxy"
	"github.com/costinm/ugate/pkg/socks"
	"github.com/costinm/ugate/pkg/udp"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		// Init DNS capture and server, as well as explicit forwarders.
		// This is a core functionality, like TCP forwarding.
		// UDP Gate is used with TProxy and lwIP.
		udp.New(ug)

		// Core functionalty - http and socks local forwarding.
		hproxy := http_proxy.NewHTTPProxy(ug)
		hproxy.HttpProxyCapture(fmt.Sprintf("127.0.0.1:%d", ug.Config.BasePort+ugate.PORT_HTTP_PROXY))

		socks.New(ug)
		return nil
	})
}
