package quic

import (
	"context"
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/costinm/meshauth"
	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/test"
)

func BenchmarkUGateQUIC(b *testing.B) {
	// Fixed key, config from filesystem. Base is 14000
	alice := test.NewTestNode(test.AliceMeshAuthCfg, &ugate.MeshSettings{BasePort: 6300})
	New(alice, &meshauth.PortListener{Address: ":6301"})

	// In memory config store. All options
	bob := test.NewTestNode(test.BobMeshAuthCfg, &ugate.MeshSettings{BasePort: 6400})
	qb := New(bob, &meshauth.PortListener{Address: ":6401"})
	qb.Start()
	log.Println(bob.Auth.VIP6)

	bobAddr := "localhost:" + strconv.Itoa(bob.BasePort+7)

	// Alice dials a MUX to bob
	bobNode, _ := alice.Cluster(nil, bob.Auth.ID)
	bobNode.Addr = bobAddr
	_, err := alice.DialMUX(context.Background(), "quic", bobNode, nil)
	if err != nil {
		b.Fatal("Error dialing mux", err)
	}

	b.Run("forward", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			con, err := alice.DialContext(context.Background(), "tcp", bob.Auth.ID+":6412")
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
			con, err := bob.DialContext(context.Background(), "tcp", alice.Auth.ID+":6412")
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

func TestQuic(t *testing.T) {

	// Fixed key, config from filesystem. Base is 14000
	alice := test.NewTestNode(test.AliceMeshAuthCfg, &ugate.MeshSettings{BasePort: 6300})
	New(alice, &meshauth.PortListener{Address: ":6307"})

	// In memory config store. All options
	bob := test.NewTestNode(test.BobMeshAuthCfg, &ugate.MeshSettings{BasePort: 6400})
	qb := New(bob, &meshauth.PortListener{Address: ":6407"})
	bob.Start()
	qb.Start()


	bobAddr := "localhost:" + strconv.Itoa(bob.BasePort+7)

	// Add a cluster to alice with bob's ID, address and protocol
	bobNode, _ := alice.Cluster(nil, bob.Auth.ID)
	bobNode.Addr = bobAddr
	bobNode.Proto = "quic"

	//_, err := alice.DialMUX(context.Background(), "quic", bobNode, nil)
	//if err != nil {
	//	t.Fatal("Error dialing mux", err)
	//}

	// Alice -> QUIC -> Bob -> Echo
	t.Run("egress", func(t *testing.T) {
		// Using DialContext interface - mesh address will use the node.
		// The port is the echo port on Bob.
		con, err := alice.DialContext(context.Background(), "tcp", bob.Auth.ID+":6412")
		if err != nil {
			t.Fatal(err)
		}
		_, err = test.CheckEcho(con, con)
	})

	//t.Run("egress10", func(t *testing.T) {
	//	for i := 0; i < 10; i++ {
	//		// Using DialContext interface - mesh address will use the node.
	//		con, err := alice.DialContext(context.Background(), "tcp", bob.Auth.ID+":6412")
	//		if err != nil {
	//			t.Fatal(err)
	//		}
	//		_, err = test.CheckEcho(con, con)
	//	}
	//})
	//
	//t.Run("egress10p", func(t *testing.T) {
	//	ch := make(chan string, 10)
	//	for i := 0; i < 10; i++ {
	//		go func() {
	//			// Using DialContext interface - mesh address will use the node.
	//			con, err := alice.DialContext(context.Background(), "tcp", bob.Auth.ID+":6412")
	//			if err != nil {
	//				t.Fatal(err)
	//			}
	//			_, err = test.CheckEcho(con, con)
	//			ch <- ""
	//		}()
	//	}
	//	for i := 0; i < 10; i++ {
	//		<-ch
	//	}
	//})

	// Bob -> H3R -> Alice -> Echo
	// Bob did not dial Alice, doesn't have the address ( and alice didn't start server )
	t.Run("reverse", func(t *testing.T) {
		// Using DialContext interface - mesh address will use the node.
		ctx, cf := context.WithTimeout(context.Background(), 5000*time.Second)
		defer cf()
		con, err := bob.DialContext(ctx, "tcp", alice.Auth.ID+":6412")
		if err != nil {
			t.Fatal(err)
		}
		_, err = test.CheckEcho(con, con)
		if err != nil {
			t.Error(err)
		}
	})

}
