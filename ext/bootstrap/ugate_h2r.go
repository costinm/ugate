// +build !MIN

package bootstrap

import (
	"github.com/costinm/ugate/ext/h2r"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		// Registers 'h2r' as a mux dialer, /h2r/ as a handler
		h2r.New(ug)
		return nil
	})
}
