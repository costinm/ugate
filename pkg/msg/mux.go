package msg

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/costinm/ugate"
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

func NewPubsub() *Pubsub {
	return &Pubsub{}
}

func (*Pubsub) Publish(ctx context.Context, cmdS string, meta map[string]string, data []byte) error {
	return nil
}

func (*Pubsub) OnMessage() {

}


// Handles incoming pubusb messages.
// 4xx, 5xx - message will be retried.
func (gw *Pubsub) HandleMsg(w http.ResponseWriter, r *http.Request) {
	var m ugate.PubSubMessage
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("ioutil.ReadAll: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &m); err != nil {
		log.Printf("json.Unmarshal: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := string(m.Message.Data)
	log.Print(r.Header, string(body), name)
}

func (gw *Pubsub) SendMsg(dst string, meta map[string]string, data []byte) {

}
