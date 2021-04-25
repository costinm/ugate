package webrtc

import (
	"testing"
	"time"

	"github.com/costinm/ugate/test"
)

func TestRTC(t *testing.T) {

	alice := test.InitTestServer(test.ALICE_KEYS, nil, nil)
	// Enable RTC for alice
	pc1, off1, err := InitWebRTCS(alice, alice.Auth)
	if err != nil {
		t.Fatal(err)
	}

	bob := test.InitTestServer(test.BOB_KEYS, nil, nil)
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

	time.Sleep(10* time.Second)
}
