//go:build !MIN
// +build !MIN

package ugated

import (
	"os"

	"github.com/costinm/ugate/pkg/local"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
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
