package quic

import (
	"context"
	"testing"
	"time"

	"github.com/costinm/ugate/pkg/test"
)

func BenchmarkUGateQUIC(b *testing.B) {
	// Fixed key, config from filesystem. Base is 14000
	ctx := context.Background()

	//alice := test.NewTestNode(test.AliceMeshAuthCfg,6300)

	qa := New()
	qa.Address = ":6402"
	qa.Provision(ctx)

	// In memory config store. All options
	qb := New()
	qb.Address = ":6401"
	qb.Provision(ctx)
	qb.Start(ctx)

	//log.Println(bob.Mesh.VIP6)

	bobAddr := "localhost:6401"
	aliceAddr := "localhost:6402"


	b.Run("forward", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			con, err := qa.DialContext(context.Background(), "tcp", bobAddr)
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
			con, err := qb.DialContext(context.Background(), "tcp", aliceAddr)
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
	ctx := context.Background()
	qa := New()
	qa.Address = ":6402"
	qa.Provision(ctx)

	// In memory config store. All options
	qb := New()
	qb.Address = ":6401"
	qb.Provision(ctx)
	qb.Start(ctx)

	//log.Println(bob.Mesh.VIP6)
	bobAddr := "localhost:6401"
	aliceAddr := "localhost:6402"


	// Alice -> QUIC -> Bob -> Echo
	t.Run("egress", func(t *testing.T) {
		// Using DialContext interface - mesh address will use the node.
		// The port is the echo port on Bob.
		con, err := qa.DialContext(context.Background(), "tcp", bobAddr)
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
		con, err := qb.DialContext(ctx, "tcp", aliceAddr)
		if err != nil {
			t.Fatal(err)
		}
		_, err = test.CheckEcho(con, con)
		if err != nil {
			t.Error(err)
		}
	})

}
