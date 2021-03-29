package webrtc

import (
	"testing"
	"time"
)

func TestRTC(t *testing.T) {

	pc1, off1, err := InitWebRTCS()
	if err != nil {
		t.Fatal(err)
	}

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
