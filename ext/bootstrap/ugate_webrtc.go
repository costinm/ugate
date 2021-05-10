// +build !MIN

package bootstrap

import (
	"os"

	"github.com/costinm/ugate/ext/webrtc"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		if os.Getenv("UGATE_RTC") == "" {
			return nil
		}
		go webrtc.InitWebRTCS(ug, ug.Auth)
		return nil
	})
}
