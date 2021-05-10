// +build !MIN

package bootstrap

import (
	"os"

	"github.com/costinm/ugate/pkg/local"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

func init() {
	initHooks = append(initHooks, func(ug *ugatesvc.UGate) startFunc {
		if os.Getenv("UGATE_LOCAL") == "" {
			return nil
		}
		// Discover local nodes using multicast UDP
		localgw := local.NewLocal(ug, ug.Auth)
		local.ListenUDP(localgw)
		go localgw.PeriodicThread()
		ug.Mux.HandleFunc("/dmesh/ll/if", localgw.HttpGetLLIf)
		return nil
	})
}
