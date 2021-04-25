// +build !MIN

package main

import (
	"github.com/costinm/ugate/pkg/ugatesvc"
)

func init() {
	initHooks = append(initHooks, func(ug *ugatesvc.UGate) startFunc{
		return nil
	})
}
