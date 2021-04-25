// +build !MIN

package main

import (
	"github.com/costinm/ugate/ext/quic"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

func init() {
	initHooks = append(initHooks, func(ug *ugatesvc.UGate) startFunc {
		qa := quic.New(ug)

		return func(ug *ugatesvc.UGate) {
			qa.Start()
		}
	})
}
