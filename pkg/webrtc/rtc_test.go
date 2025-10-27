package webrtc

import (
	"context"
	"log"
	"testing"
	"time"
)

func TestRTC(t *testing.T) {
	ctx := context.Background()

	alice := &RTConn{Name: "alice", Client: true}

	err := alice.Dial()
	// Enable RTConn for alice
	if err != nil {
		t.Fatal(err)
	}

	// Server side takes the client 'invite' - including cert sha.
	bob := &WebRTC{Name: "bob"}
	err = bob.Provision(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Normally the 'client hello' is sent using HTTP or other channel, server response
	// is set back on the client.
	bobSessionDesc, err := bob.AcceptPeering(alice.offer)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for the answer to be submitted via HTTP
	//answer := webrtc.SessionDescription{}
	//Decode(res, &answer)

	// Set the remote SessionDescription
	err = alice.pc1.SetRemoteDescription(*bobSessionDesc)
	if err != nil {
		t.Fatal(err)
	}
	log.Println("Bob:", bobSessionDesc)

	//alice.DataChannel("ch2")

	// At this point alice and bob should be able to communicate

	time.Sleep(10 * time.Second)
}
