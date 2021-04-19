package main

import (
	"os"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/costinm/ugate/quic"
)

func init() {
	initHooks = append(initHooks, func(ug *ugatesvc.UGate) startFunc {
		// We will only register a single QUIC server by default, and a factory for cons
		port := ug.Config.BasePort + ugate.PORT_HTTPS
		if os.Getuid() == 0 {
			port = 443
		}
		quic.InitQuicServer(ug.Auth, port, ug.H2Handler)

		quic.InitMASQUE(ug.Auth, ug.Config.BasePort + ugate.PORT_BTS, ug, ug)

		return nil
	})
}
