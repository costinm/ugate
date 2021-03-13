package msgs

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

	"github.com/costinm/ugate/pkg/auth"
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

// Mux handles processing messages for this node, and sending messages from
// local code.
type Mux struct {
	mutex sync.RWMutex

	// MessageSenders tracks all connections that support send to the remote end.
	// For example UDS connections, SSH, etc.
	connections map[string]*MsgConnection

	// Handlers by path, for processing incoming messages to this node or local messages.
	handlers     map[string]MessageHandler
	handlerRoles map[string][]string

	// Allows regular HTTP Handlers to process messages.
	// A message is mapped to a request. Like CloudEvents, response from the
	// http request can be mapped to a Send (not supported yet).
	ServeMux *http.ServeMux

	// Auth holds the private key and Id of this node. Used to encrypt and decrypt.
	Auth *auth.Auth

	OnMessageForNode []OnMessage
}

type OnMessage func(*Message)

func NewMux() *Mux {
	mux := &Mux{
		connections:  map[string]*MsgConnection{},
		handlers:     map[string]MessageHandler{},
		handlerRoles: map[string][]string{},
	}

	return mux
}

var DefaultMux = NewMux()

// Send a message to the default mux. Will serialize the event and save it for debugging.
//
// Local handlers and debug tools/admin can subscribe.
func Send(msgType string, meta ...string) {
	DefaultMux.Send(msgType, nil, meta...)
}

//
func (mux *Mux) Send(msgType string, data interface{}, meta ...string) error {
	ev := &Message{
		MessageData: MessageData {
			To: msgType,
			Meta: map[string]string{},
		},
		Data: data,
	}
	for i := 0; i < len(meta); i += 2 {
		ev.Meta[meta[i]] = meta[i+1]
	}
	return mux.SendMessage(ev)
}

var (
	id    int
	mutex sync.Mutex
)

// Publish a message. Will be distributed to remote listeners.
// TODO: routing for directed messages (to specific destination)
// TODO: up/down indication for multicast, subscription
func (mux *Mux) SendMessage(ev *Message) error {
	_ = context.Background()
	// Local handlers first
	if ev.Id == "" {
		mutex.Lock()
		ev.Id = fmt.Sprintf("%d", id)
		id++
		mutex.Unlock()
	}
	mux.HandleMessageForNode(ev)

	return mux.SendMsg(ev)
}

// Called for local events (host==. or empty).
// Called when a message is received from one of the local streams ( UDS, etc ), if
// the final destination is the current node.
//
// Message will be passed to one or more of the local handlers, based on type.
//
// TODO: authorization (based on identity of the caller)
func (mux *Mux) HandleMessageForNode(ev *Message) error {
	if ev.Time== 0 {
		ev.Time = time.Now().Unix()
	}

	for _, cb := range mux.OnMessageForNode {
		cb(ev)
	}

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

	if r := mux.handlerRoles[topic]; r != nil {
		// Check From
	}

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

// Add a local handler for a specific message type.
// Special topics: *, /open, /close
func (mux *Mux) AddHandler(path string, cp MessageHandler) {
	mux.mutex.Lock()
	mux.handlers[path] = cp
	mux.mutex.Unlock()
}

// Add a handler that checks the role
func (mux *Mux) AddHandlerRole(path string, role ...string) {
	mux.mutex.Lock()
	mux.handlerRoles[path] = role
	mux.mutex.Unlock()
}

type ChannelHandler struct {
	MsgChan chan *Message
}

func NewChannelHandler() *ChannelHandler {
	return &ChannelHandler{MsgChan: make(chan *Message, 100)}
}

func (u *ChannelHandler) HandleMessage(ctx context.Context, cmdS string, meta map[string]string, data []byte) {
	log.Println("MSG: ", cmdS)
	m := NewMessage(cmdS, meta).SetDataJSON(data)
	//m.Connection = replyTo
	u.MsgChan <- m
}

func (u *ChannelHandler) WaitEvent(name string) *Message {
	tmax := time.After(8 * time.Second)

	for {
		select {
		case <-tmax:
			return nil
		case e := <-u.MsgChan:
			if e.To == name {
				return e
			}
			if strings.HasPrefix(e.To, name) {
				return e
			}
			log.Println("EVENT", e)
		}
	}

	return nil
}
