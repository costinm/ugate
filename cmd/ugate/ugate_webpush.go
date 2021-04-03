package main

import (
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/costinm/ugate/webpush"
)

func init() {
	initHooks = append(initHooks, func(gate *ugatesvc.UGate) startFunc {
		webpush.InitMux(webpush.DefaultMux, gate.Mux, gate.Auth)
		return nil
	})
}
