package webrtc

import (
	"testing"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/test"
)

func TestRTC(t *testing.T) {

	alice := test.NewTestNode(test.AliceMeshAuthCfg, &ugate.MeshSettings{
		BasePort: 5700,
	})
	// Enable RTC for alice
	pc1, off1, err := InitWebRTCS(alice, alice.Auth)
	if err != nil {
		t.Fatal(err)
	}

	bob := test.NewTestNode(test.BobMeshAuthCfg, &ugate.MeshSettings{
		BasePort: 5800,
	})
	InitWebRTCS(bob, bob.Auth)

	res, err := DialWebRTC(off1)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for the answer to be submitted via HTTP
	//answer := webrtc.SessionDescription{}
	//Decode(res, &answer)

	// Set the remote SessionDescription
	err = pc1.SetRemoteDescription(*res)
	if err != nil {
		panic(err)
	}

	time.Sleep(10 * time.Second)
}
