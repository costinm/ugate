package mqtt

import (
	"log"

	mqtt "github.com/mochi-co/mqtt/server"
	"github.com/mochi-co/mqtt/server/listeners/auth"

	"github.com/mochi-co/mqtt/server/listeners"
)

func InitMqtt() {
	options := &mqtt.Options{
		BufferSize:      0,       // Use default values
		BufferBlockSize: 0,       // Use default values
		InflightTTL:     60 * 15, // Set an example custom 15-min TTL for inflight messages
	}

	server := mqtt.NewServer(options)
	tcp := listeners.NewTCP("t1", ":1883")
	err := server.AddListener(tcp, &listeners.Config{
		Auth: new(auth.Allow),
	})
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()
}
