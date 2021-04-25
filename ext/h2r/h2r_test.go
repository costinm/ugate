package h2r

import (
	"context"
	"log"
	"strconv"
	"testing"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/costinm/ugate/test"
)

func TestH2R(t *testing.T) {

	// Fixed key, config from filesystem. Base is 14000
	alice := test.InitTestServer(test.ALICE_KEYS, &ugate.GateCfg{BasePort: 6300, Name: "alice"}, func(gate *ugatesvc.UGate) {
		New(gate)
	})

	// In memory config store. All options
	bob := 	test.InitTestServer(test.BOB_KEYS, &ugate.GateCfg{BasePort: 6400, Name: "bob"}, func(bob *ugatesvc.UGate) {
		New(bob)
	})
	log.Println(bob.Auth.VIP6)

	bobAddr := "localhost:" + strconv.Itoa(bob.Config.BasePort + ugate.PORT_BTS)

	// Alice dials a MUX to bob
	bobNode := alice.GetOrAddNode(bob.Auth.ID)
	bobNode.Addr = bobAddr
	_, err := alice.DialMUX(context.Background(), "h2r", bobNode, nil)
	if err != nil {
		t.Fatal("Error dialing mux", err)
	}


	// Alice -> QUIC -> Bob -> Echo
	t.Run("egress", func(t *testing.T) {
		// Using DialContext interface - mesh address will use the node.
		con, err := alice.DialContext(context.Background(), "tcp", bob.Auth.ID + ":6412")
		if err != nil {
			t.Fatal(err)
		}
		_, err = test.CheckEcho(con, con)
	})

	// Bob -> H3R -> Alice -> Echo
	// Bob did not dial Alice, doesn't have the address ( and alice didn't start server )
	t.Run("reverse", func(t *testing.T) {
		// Using DialContext interface - mesh address will use the node.
		con, err := bob.DialContext(context.Background(), "tcp", alice.Auth.ID + ":6412")
		if err != nil {
			t.Fatal(err)
		}
		_, err = test.CheckEcho(con, con)
	})

}
