package main

import (
	"os"

	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/costinm/ugate/webrtc"
)

func init() {
	initHooks = append(initHooks, func(ug *ugatesvc.UGate) startFunc {
		if os.Getenv("UGATE_RTC") == "" {
			return nil
		}
		go webrtc.InitWebRTCS(ug, ug.Auth)
		return nil
	})
}
