package webpush

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/costinm/meshauth"
)

//Old: go:generate protoc --gogofaster_out=$GOPATH/src webpush.proto

// Local processing of messages. Interface doesn't use any specific struct,
// to avoid creating deps.
type MessageHandler interface {
	// Handle a message. Context may provide access to the actual message object
	// and mux.
	HandleMessage(ctx context.Context, cmdS string, meta map[string]string, data []byte)
}

// Adapter from func to interface
type HandlerCallbackFunc func(ctx context.Context, cmdS string, meta map[string]string, data []byte)

// ServeHTTP calls f(w, r).
func (f HandlerCallbackFunc) HandleMessage(ctx context.Context, cmdS string, meta map[string]string, data []byte) {
	f(ctx, cmdS, meta, data)
}

var (
	id    int
	mutex sync.Mutex
)

// Mux handles processing messages for this node, and sending messages from
// local code.
type Mux struct {
	mutex sync.RWMutex

	// MessageSenders tracks all connections that support send to the remote end.
	// For example UDS connections, SSH, etc.
	connections map[string]*MsgConnection

	// Handlers by path, for processing incoming messages to this node or local messages.
	handlers map[string]MessageHandler

	// Allows regular HTTP Handlers to process messages.
	// A message is mapped to a request. Like CloudEvents, response from the
	// http request can be mapped to a Send (not supported yet).
	ServeMux *http.ServeMux

	// Auth holds the private key and muxID of this node. Used to encrypt and decrypt.
	Auth *meshauth.MeshAuth
}

func NewMux() *Mux {
	mux := &Mux{
		connections: map[string]*MsgConnection{},
		handlers:    map[string]MessageHandler{},
	}

	return mux
}

var DefaultMux = NewMux()

func nextId() string {
	mutex.Lock()
	defer mutex.Unlock()
	id++
	return fmt.Sprintf("%d", id)
}

// Init a mux with a http.Transport
// Normlly there is a single mux in a server - multiple mux
// are used for testing.
func InitMux(mux *Mux, hmux *http.ServeMux, auth *meshauth.MeshAuth) {
	mux.Auth = auth
	hmux.HandleFunc("/push/", mux.HTTPHandlerWebpush)
	hmux.HandleFunc("/subscribe", mux.SubscribeHandler)
	hmux.HandleFunc("/s/", mux.HTTPHandlerSend)
}

// Send a message to the default mux. Will serialize the event and save it for debugging.
//
// Local handlers and debug tools/admin can subscribe.
// Calls the internal SendMessage method.
func (mux *Mux) Send(msgType string, data interface{}, meta ...string) error {
	ev := &Message{
		MessageData: MessageData{
			To:   msgType,
			Meta: map[string]string{},
		},
		Data: data,
	}
	for i := 0; i < len(meta); i += 2 {
		ev.Meta[meta[i]] = meta[i+1]
	}
	return mux.SendMessage(ev)
}

// Send a message to the default mux. Will serialize the event and save it for debugging.
//
// Local handlers and debug tools/admin can subscribe.
// Calls the internal SendMessage method.
func (mux *Mux) SendMeta(msgType string, meta map[string]string, data interface{}) error {
	ev := &Message{
		MessageData: MessageData{
			To:   msgType,
			Meta: meta,
		},
		Data: data,
	}
	return mux.SendMessage(ev)
}

// Publish a message. Will be distributed to local and remote listeners.
//
// TODO: routing for directed messages (to specific destination)
// TODO: up/down indication for multicast, subscription
func (mux *Mux) SendMessage(ev *Message) error {
	_ = context.Background()

	// Local handlers first
	if ev.Id == "" {
		ev.Id = nextId()
	}

	parts := strings.Split(ev.To, "/")

	mux.HandleMessageForNode(ev)

	switch parts[0] {
	case ".":
		return nil
	case "*":

	default:
		h := parts[0]
		if h != "" {
			ch := mux.connections[h]
			if ch != nil {
				ch.SendMessageToRemote(ev)
			} else {
				// TODO: return err or send to 'master'
			}
		}
	}

	if parts[0] == "." {
		return nil
	}
	if len(parts) < 2 {
		return nil
	}

	for k, ms := range mux.connections {
		if k == ev.From { // Exclude the connection where this was received on.
			continue
		}
		log.Println("/mux/SendFWD", ev.To, k)
		ms.maybeSend(parts, ev, k)
	}
	// Send upstream
	return nil
}

func (ms *MsgConnection) maybeSend(parts []string, ev *Message, k string) {
	// TODO: check the path !
	if parts[0] != "" {
		// TODO: send if the peer WorkloadID matches, or if peer has sent a (signed) event message that the node
		// is connected
	}

	if ms.SubscriptionsToSend == nil {
		return
	}
	//if Debug {
	//	log.Println("MSG: fwd to connection ", ev.To, k, ms.Name)
	//}
	topic := parts[1]
	hasSub := false
	for _, s := range ms.SubscriptionsToSend {
		if topic == s || s == "*" {
			hasSub = true
			break
		}
	}
	if !hasSub {
		return
	}

	ms.SendMessageToRemote(ev)
	log.Println("/mux/Remote", ev.To, ms.Name)
}

// Called for local events (host==. or empty).
// Called when a message is received from one of the local streams ( UDS, etc ), if
// the final destination is the current node.
//
// Message will be passed to one or more of the local handlers, based on type.
func (mux *Mux) HandleMessageForNode(ev *Message) error {
	if ev.Time == 0 {
		ev.Time = time.Now().Unix()
	}

	//for _, cb := range mux.OnMessageForNode {
	//	cb(ev)
	//}

	//log.Println("EV: ", ev.To, ev.From)
	if ev.To == "" {
		return nil
	}

	argv := strings.Split(ev.To, "/")

	if len(argv) < 2 {
		return nil
	}

	toNode := argv[0]
	if toNode != "" && toNode != mux.Auth.VIP6.String() {
		// Currently local handlers only support local originated messages.
		// Use a connection for full support.
		return nil
	}
	topic := argv[1]

	payload := ev.Binary()
	log.Println("MSG: ", argv, ev.Meta, ev.From, ev.Data, len(payload))

	if h, f := mux.handlers["*"]; f {
		h.HandleMessage(context.Background(), ev.To, ev.Meta, payload)
	}

	if mux.ServeMux != nil {
		r := &http.Request{
			URL: &url.URL{
				Path: ev.To,
			},
			Host: argv[0],
		}
		h, p := mux.ServeMux.Handler(r)
		if h != nil && p != "/" {
			log.Println("Server handler: ", ev.To)
			w := &rw{}
			h.ServeHTTP(w, r)
		}
		return nil
	}

	if h, f := mux.handlers[topic]; f {
		h.HandleMessage(context.Background(), ev.To, ev.Meta, payload)
	} else if h, f = mux.handlers[""]; f {
		log.Println("UNHANDLED: ", ev.To)
		h.HandleMessage(context.Background(), ev.To, ev.Meta, payload)
	}
	return nil
}

type rw struct {
	Code      int
	HeaderMap http.Header
	Body      *bytes.Buffer
}

func newHTTPWriter() *rw {
	return &rw{}
}

func (r *rw) Header() http.Header {
	m := r.HeaderMap
	if m == nil {
		m = make(http.Header)
		r.HeaderMap = m
	}
	return m
}

func (r *rw) Write(d []byte) (int, error) {
	return r.Body.Write(d)
}

func (r *rw) Flush([]byte) {
}

func (r *rw) WriteHeader(statusCode int) {
	r.Code = statusCode
}

// StartListener a local handler for a specific message type.
// Special topics: *, /open, /close
func (mux *Mux) AddHandler(path string, cp MessageHandler) {
	mux.mutex.Lock()
	old := mux.handlers[path]
	if old == nil {
		mux.handlers[path] = cp
	} else {
		if os, ok := old.(handlerSlice); ok {
			os = append(os, cp)
			mux.handlers[path] = os
		} else {
			hs := []MessageHandler{old, cp}
			mux.handlers[path] = handlerSlice(hs)
		}
	}
	log.Println("AddHandler", path, mux.handlers[path])
	mux.mutex.Unlock()
}

type handlerSlice []MessageHandler

func (hs handlerSlice) HandleMessage(ctx context.Context, cmdS string, meta map[string]string, data []byte) {
	for _, x := range hs {
		x.HandleMessage(ctx, cmdS, meta, data)
	}
}
