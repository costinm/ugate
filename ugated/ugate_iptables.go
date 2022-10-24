//go:build !MIN
// +build !MIN

package ugated

import (
	"fmt"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/iptables"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

// Istio style iptables on 15001 (out) and 15006 (in)
func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		// Init Iptables capture (off by default - android doesn't like it)
		// Not on localhost - redirect changes the port, keeps IP
		go iptables.IptablesCapture(ug, fmt.Sprintf("0.0.0.0:%d", ug.Config.BasePort+ugate.PORT_IPTABLES), false)
		go iptables.IptablesCapture(ug, fmt.Sprintf("0.0.0.0:%d", ug.Config.BasePort+ugate.PORT_IPTABLES_IN), true)
		return nil
	})
}
