package ugated

import (
	"github.com/costinm/ugate"

	"github.com/costinm/ugate/pkg/webrtc"
)

func init() {
	ugate.Modules["webrtc"] = func(ug *ugate.UGate) {
		go webrtc.InitWebRTCS(ug, ug.Auth)
	}
}
