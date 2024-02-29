package ugated

import (
	"fmt"

	"github.com/costinm/meshauth"
	"github.com/costinm/ugate"
	mqtt "github.com/mochi-co/mqtt/server"
	"github.com/mochi-co/mqtt/server/events"
	"github.com/mochi-co/mqtt/server/listeners"
	"github.com/mochi-co/mqtt/server/listeners/auth"
	"golang.org/x/exp/slog"
)

type MqttServer struct {
}

// mochi is a server for mqtt, using bolt DB for persistence.
// paho is the client. Depends on gorilla/websocket only.

/*
Other servers exist - for example:
 - mosquitto
 -emqx - with gateways to stomp,coap,LwM2M, http
	- multiple brokers, 2M/node
    - API to list nodes, load info for each
    - API for clients, subs
    - /api/v4/mqtt/publish -topic(s),clientid,payload,retain,properties(expiry)
 - nanomq - bridge with cloud mqtt, zeromq, nanomsg,nng
*/

func init() {
	ugate.Modules["mqtt"] = func(gate *ugate.UGate) {
		gate.ListenerProto["mqtt"] = func(gate *ugate.UGate, ll *meshauth.PortListener) error {
			options := &mqtt.Options{
				BufferSize:      0,       // Use default values
				BufferBlockSize: 0,       // Use default values
				InflightTTL:     60 * 15, // Set an example custom 15-min TTL for inflight messages
			}

			server := mqtt.NewServer(options)

			// TODO: all hbone listeners, direct
			tcp := listeners.NewTCP("t1", ll.Address)
			err := server.AddListener(tcp, &listeners.Config{
				Auth: new(auth.Allow),
			})
			if err != nil {
				slog.Info("starting mqtt", "err", err)
				return err
			}

			server.Events.OnConnect = func(client events.Client, packet events.Packet) {
				slog.Info("mqtt-connect", "id", client.ID,
					"packet", packet)
			}
			server.Events.OnDisconnect = func(client events.Client, err error) {
				slog.Info("mqtt-connect", "id", client.ID,
					"err", err)
			}

			server.Events.OnSubscribe = func(filter string, cl events.Client, qos byte) {
				fmt.Printf("<< OnSubscribe client subscribed %s: %s %v\n", cl.ID, filter, qos)
			}

			server.Events.OnUnsubscribe = func(filter string, cl events.Client) {
				fmt.Printf("<< OnUnsubscribe client unsubscribed %s: %s\n", cl.ID, filter)
			}

			// Add OnMessage Event Hook
			server.Events.OnMessage = func(cl events.Client, pk events.Packet) (pkx events.Packet, err error) {
				pkx = pk
				if string(pk.Payload) == "hello" {
					pkx.Payload = []byte("hello world")
					fmt.Printf("< OnMessage modified message from client %s: %s\n", cl.ID, string(pkx.Payload))
				} else {
					fmt.Printf("< OnMessage received message from client %s: %s\n", cl.ID, string(pkx.Payload))
				}

				// Example of using AllowClients to selectively deliver/drop messages.
				// Only a client with the id of `allowed-client` will received messages on the topic.
				if pkx.TopicName == "a/b/restricted" {
					pkx.AllowClients = []string{"allowed-client"} // slice of known client ids
				}

				return pkx, nil
			}

			server.Publish("direct/publish", []byte("scheduled message"), true)

			go func() {
				err := server.Serve()
				if err != nil {
					slog.Info("starting mqtt", "err", err)
				}

			}()
			return nil
		}
	}
}
