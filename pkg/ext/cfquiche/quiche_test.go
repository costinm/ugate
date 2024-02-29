package cfquiche

import (
	"context"
	"log"
	"strconv"
	"testing"

	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/costinm/ugate/pkg/test"
)

func BenchmarkUGateQUIC(b *testing.B) {
	// Fixed key, config from filesystem. Base is 14000
	alice := test.InitTestServer(test.ALICE_KEYS, &ugatesvc.MeshSettings{BasePort: 6300, AccessLog: true, Name: "alice"}, func(gate *ugate.UGate) {
		New(gate)
	})

	// In memory config store. All options
	bob := test.InitTestServer(test.BOB_KEYS, &ugatesvc.MeshSettings{BasePort: 6400, AccessLog: true, Name: "bob"}, func(bob *ugate.UGate) {
		New(bob)
	})
	log.Println(bob.Auth.VIP6)

	bobAddr := "localhost:" + strconv.Itoa(bob.Config.BasePort+ugatesvc.PORT_BTS)

	// Alice dials a MUX to bob
	bobNode := alice.GetOrAddNode(bob.Auth.ID)
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
	alice := test.InitTestServer(test.ALICE_KEYS, &ugatesvc.MeshSettings{BasePort: 6300, Name: "alice"}, func(gate *ugate.UGate) {
		New(gate)
	})

	//// In memory config store. All options
	//bob := 	test.NewTestNode(test.BOB_KEYS, &ugate.MeshSettings{BasePort: 6400, Name: "bob"}, func(bob *ugate.UGate) {
	//	New(bob)
	//})
	//log.Println(bob.Auth.VIP6)

	bobAddr := "localhost:" + strconv.Itoa(6100+ugatesvc.PORT_BTS)

	// Alice dials a MUX to bob
	bobNode := alice.GetOrAddNode(test.BOB_ID)
	bobNode.Addr = bobAddr
	_, err := alice.DialMUX(context.Background(), "quiche", bobNode, nil)
	if err != nil {
		t.Fatal("Error dialing mux", err)
	}

	// Alice -> QUIC -> Bob -> Echo
	t.Run("egress", func(t *testing.T) {
		// Using DialContext interface - mesh address will use the node.
		con, err := alice.DialContext(context.Background(), "tcp", test.BOB_ID+":6412")
		if err != nil {
			t.Fatal(err)
		}
		_, err = test.CheckEcho(con, con)
	})

	//// Bob -> H3R -> Alice -> Echo
	//// Bob did not dial Alice, doesn't have the address ( and alice didn't start server )
	//t.Init("reverse", func(t *testing.T) {
	//	// Using DialContext interface - mesh address will use the node.
	//	ctx, cf := context.WithTimeout(context.Background(), 5000 * time.Second)
	//	defer cf()
	//	con, err := bob.DialContext(ctx, "tcp", alice.Auth.WorkloadID + ":6412")
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//	_, err = test.CheckEcho(con, con)
	//	if err != nil {
	//		t.Error(err)
	//	}
	//})

}
