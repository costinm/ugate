//go:build !MIN
// +build !MIN

package ugated

import (
	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/local"
)

// Multicast discovery of local nodes.
//
func init() {
	ugate.Modules["local"] = func(ug *ugate.UGate) {
		// Discover local nodes using multicast UDP
		if len(ug.Auth.PublicKey) != 65 {
			// Only works for EC256 keys
			return
		}
		localgw := local.NewLocal(ug.BasePort+8, ug.Auth)
		local.ListenUDP(localgw)
		ug.Mux.HandleFunc("/dmesh/ll/if", localgw.HttpGetLLIf)
	}
}
