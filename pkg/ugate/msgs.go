package ugate

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/costinm/ugate"
)



// Handles incoming pubusb messages.
// 4xx, 5xx - message will be retried.
func (gw *UGate) HandleMsg(w http.ResponseWriter, r *http.Request) {
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

func (gw *UGate) SendMsg(dst string, meta map[string]string, data []byte) {

}
