// +build !MIN

package bootstrap

import (
	"github.com/costinm/ugate/ext/quic"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		qa := quic.New(ug)

		return func(ug *ugatesvc.UGate) {
			qa.Start()
		}
	})
}
