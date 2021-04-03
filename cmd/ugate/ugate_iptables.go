package main

import (
	"fmt"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/iptables"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

func init() {
	initHooks = append(initHooks, func(ug *ugatesvc.UGate) startFunc {
		// Init Iptables capture (off by default - android doesn't like it)
		iptables.IptablesCapture(ug, fmt.Sprintf("0.0.0.0:%d", ug.Config.BasePort+ugate.PORT_IPTABLES), false)
		iptables.IptablesCapture(ug, fmt.Sprintf("0.0.0.0:%d", ug.Config.BasePort+ugate.PORT_IPTABLES_IN), true)
		return nil
	})
}
