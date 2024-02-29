package ugated

import (
	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/h2r"
)

func init() {
	ugate.Modules["h2r"] = func(ug *ugate.UGate) {
		// Registers 'h2r' as a mux dialer, /h2r/ as a handler
		h2r.New(ug)
	}
}
