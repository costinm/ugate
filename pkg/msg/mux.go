package msg

import (
	"context"
	"net/http"
)

// - Ugate has a list of reverse H2 clients
// - we have discovery to locate push dest IP
// What is missing is a 'subscription', 'publish' and
// the light dispatching, plus integration with other brokers.

type Pubsub struct {
	// Messages are mapped to HTTP request with URL /msg/TOPIC
	// Long lived connections register as /sub/SUBID
	Mux http.ServeMux

	Topics map[string]*Topic

}

type Topic struct {
	Name string

	// Upstream brokers
	Brokers []Sub

	Subs []*Sub
}

type Sub struct {

	ID string

	// URL - if set, the message will be posted.
	Dest string

	HTTPHandler http.Handler
}

func (*Pubsub) Publish(ctx context.Context, cmdS string, meta map[string]string, data []byte) error {
	return nil
}

func (*Pubsub) OnMessage() {

}
