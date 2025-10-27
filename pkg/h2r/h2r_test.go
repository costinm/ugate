package h2r

import (
	"context"
	"log"
	"testing"

	"github.com/costinm/ugate/pkg/test"
)

func BenchmarkUGateH2R(b *testing.B) {
	// Fixed key, config from filesystem. Base is 14000
	alice := test.NewTestNode("../../testdata/alice")
	New(alice)

	// In memory config store. All options
	bob := test.NewTestNode("../../testdata/alice")
	New(bob)
	//log.Println(bob.Mesh.VIP6)

	bobAddr := "localhost:6407"

	// A basic echo server - echo on 5012
	test.InitEcho(6000)

	// Alice dials a MUX to bob
	bobNode, _ := alice.Discover(nil, bob.ID)
	bobNode.Addr = bobAddr
	_, err := alice.DialMUX(context.Background(), "h2r", bobNode, nil)
	if err != nil {
		b.Fatal("Error dialing mux", err)
	}

	b.Run("forward", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			con, err := alice.DialContext(context.Background(), "tcp", bob.Mesh.ID+":6012") // ":6412")
			if err != nil {
				b.Fatal(err)
			}
			_, err = test.CheckEcho(con, con)
		}
	})

	b.Run("reverse", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			//ctx, cf := context.WithTimeout(context.Background(), 5 * time.Second)
			//defer cf()
			con, err := bob.DialContext(context.Background(), "tcp", alice.Mesh.ID+":6012")
			if err != nil {
				b.Fatal(err)
			}
			_, err = test.CheckEcho(con, con)
			if err != nil {
				b.Log("Error", err)
			}
		}
	})
}

func TestH2R(t *testing.T) {
	// A basic echo server - echo on 5012
	test.InitEcho(6000)

	// Fixed key, config from filesystem. Base is 14000
	alice := test.NewTestNode(test.AliceMeshAuthCfg, 6300)
	New(alice)

	// In memory config store. All options
	bob := test.NewTestNode(test.BobMeshAuthCfg, 6400)
	New(bob)
	log.Println(bob.Mesh.VIP6)

	bobAddr := "localhost:6407"

	// Alice dials a MUX to bob
	bobNode, _ := alice.Mesh.GetOrAddDest(nil, bob.Mesh.ID)
	bobNode.Addr = bobAddr

	_, err := alice.DialMUX(context.Background(), "h2r", bobNode, nil)
	if err != nil {
		t.Fatal("Error dialing mux", err)
	}

	// Alice -> QUIC -> Bob -> Echo
	t.Run("egress", func(t *testing.T) {
		// Using DialContext interface - mesh address will use the node.
		con, err := alice.DialContext(context.Background(), "tcp", bob.Mesh.ID+":6012") // 6412
		if err != nil {
			t.Fatal(err)
		}
		_, err = test.CheckEcho(con, con)
	})

	// Bob -> H3R -> Alice -> Echo
	// Bob did not dial Alice, doesn't have the address ( and alice didn't start server )
	t.Run("reverse", func(t *testing.T) {
		// Using DialContext interface - mesh address will use the node.
		con, err := bob.DialContext(context.Background(), "tcp", alice.Mesh.ID+":6012")
		if err != nil {
			t.Fatal(err)
		}
		_, err = test.CheckEcho(con, con)
	})

}
