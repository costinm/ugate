package main

import (
	"github.com/costinm/ugate/ext/h2r"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

func init() {
	initHooks = append(initHooks, func(ug *ugatesvc.UGate) startFunc {

		h2r.New(ug)
		return nil
	})
}
