package watermill

import (
	"context"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
)

/*
Watermill is a library integrating multiple message brokers and pub/sub systems
with a simple and flexible API.

Like K8S, it supports a concept they call CQRS (command/query separation) where writes
are handled as a command, while reads can be observed and queried from
replicas (controllers).

It has integrations with major pubsubs.

Message:
- UUID string
- Metadata map[string]string
- Payload []byte
- Context/SetContext
- ack or nack



*/


type Watermill struct {

}

// Integration with the module structure:
// - each package has a Config object
// - a NewXXX function taking config and a logger.
// - can use references to populate the constructor.

func (*Watermill) Start() {
	pubSub := gochannel.NewGoChannel(
		gochannel.Config{},
		watermill.NewStdLogger(false, false),
		)
	messages, err := pubSub.Subscribe(context.Background(), "example.topic")
	if err != nil {
		panic(err)
	}

	go process(messages)

	pubSub.Publish("example.topic", message.NewMessage("1",
		message.Payload([]byte("hi"))))


}

func process(messages <-chan *message.Message) {

}
