//go:build NOISE
// +build NOISE

package noise

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/perlin-network/noise"
	"github.com/perlin-network/noise/kademlia"
	"go.uber.org/zap"
)

/*
- dead for 5 years
- another crypto project


*/

const printedLength = 8

func New(port uint16) {
	logger, err := zap.NewDevelopment(zap.AddStacktrace(zap.PanicLevel))
	defer logger.Sync()
	log := logger.Sugar()

	alice, err := noise.NewNode(noise.WithNodeLogger(logger),
		noise.WithNodeBindPort(port))

	if err != nil {
		panic(err)
	}
	//node.RegisterMessage(chatMessage{}, unmarshalChatMessage)

	events := kademlia.Events{
		OnPeerAdmitted: func(id noise.ID) {
			log.Info("Learned about a new peer %s(%s).\n", id.Address, id.ID.String()[:printedLength])
		},
		OnPeerEvicted: func(id noise.ID) {
			log.Info("Forgotten a peer %s(%s).\n", id.Address, id.ID.String()[:printedLength])
		},
	}
	overlay := kademlia.New(kademlia.WithProtocolEvents(events))

	alice.Bind(overlay.Protocol())

	alice.Handle(func(ctx noise.HandlerContext) error {
		if !ctx.IsRequest() {
			return nil
		}

		//obj, err := ctx.DecodeMessage()
		//if err != nil {
		//	return nil
		//}
		//
		//msg, ok := obj.(chatMessage)
		//if !ok {
		//	return nil
		//}

		log.Debug("Got a message from Alice: '%s'\n", string(ctx.Data()))

		return ctx.Send([]byte("Hi Alice!"))
	})

	err = alice.Listen()
	if err != nil {
		panic(err)
	}

	//res, err := alice.Request(context.TODO(), bob.Addr(), []byte("Hi Bob!"))
	//check(err)

}

// bootstrap pings and dials an array of network addresses which we may interact with and  discover peers from.
func bootstrap(node *noise.Node, addresses ...string) {
	for _, addr := range addresses {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := node.Ping(ctx, addr)
		cancel()

		if err != nil {
			fmt.Printf("Failed to ping bootstrap node (%s). Skipping... [error: %s]\n", addr, err)
			continue
		}
	}
}

// discover uses Kademlia to discover new peers from nodes we already are aware of.
func discover(overlay *kademlia.Protocol) {
	ids := overlay.Discover()

	var str []string
	for _, id := range ids {
		str = append(str, fmt.Sprintf("%s(%s)", id.Address, id.ID.String()[:printedLength]))
	}

	if len(ids) > 0 {
		fmt.Printf("Discovered %d peer(s): [%v]\n", len(ids), strings.Join(str, ", "))
	} else {
		fmt.Printf("Did not discover any peers.\n")
	}
}

// peers prints out all peers we are already aware of.
func peers(overlay *kademlia.Protocol) {
	ids := overlay.Table().Peers()

	var str []string
	for _, id := range ids {
		str = append(str, fmt.Sprintf("%s(%s)", id.Address, id.ID.String()[:printedLength]))
	}

	fmt.Printf("You know %d peer(s): [%v]\n", len(ids), strings.Join(str, ", "))
}
