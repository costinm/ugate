//go:build !MIN
// +build !MIN

package ugated

import (
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/costinm/ugate/ugated/pkg/h2r"
)

func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		// Registers 'h2r' as a mux dialer, /h2r/ as a handler
		h2r.New(ug)
		return nil
	})
}
