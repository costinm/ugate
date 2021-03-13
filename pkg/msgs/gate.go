package msgs

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/costinm/ugate/pkg/auth"
)

// One connection - incoming or outgoing. Can send messages to the remote end, which may
// in turn forward messages for other nodes.
//
// Incoming messages are dispatched to the mux, which may deliver locally or forward.
type MsgConnection struct {
	gate *Mux

	// Key used in mux to track this connection
	Name string

	// Authenticated Vip associated with the connection. Messages will not be forwarded if
	// the VIP is in Path or From of the message.
	VIP string

	// Broadcast subscriptions to forward to the remote. Will have a 'From' set to current node.
	// VPN and upstream server use "*" to receive/pass up all events.
	// TODO: keep some messages local, by using To=., and indicate broadcasts as *.
	SubscriptionsToSend []string

	// OnMessage is called when a message for this connection is dispatched.
	// The message should be either a broadcast, have as To the vip of the connection or
	// another vip reachable from the connection.
	//
	// The topic of the message should be in the Subscription list if the destination is this vip.
	//
	// Internal handlers may use the same interface.
	SendMessageToRemote func(ev *Message) error

	Conn net.Conn
}

// id - remote id. "uds" for the primary upstream uds connection to host (android app or wifi/root app)
//
func (mux *Mux) AddConnection(id string, cp *MsgConnection) {
	cp.Name = id
	cp.gate = mux
	mux.mutex.Lock()
	mux.connections[id] = cp
	mux.mutex.Unlock()

	if mux.Auth != nil {
		// Special message sent at connect time: /I (identity)
		b64 := base64.URLEncoding.EncodeToString(mux.Auth.NodeID())

		// TODO: change to /I, with id as param ?
		cp.SendMessageToRemote(NewMessage("/I/"+b64, nil))
	}
	// Notify any handler of a new connection
	if h, f := mux.handlers["/open"]; f {
		h.HandleMessage(context.Background(), "/open", map[string]string{"id": id}, nil)
	}
	log.Println("/mux/AddConnection", id, cp.VIP, cp.SubscriptionsToSend)
}

func (cp *MsgConnection) send(message *Message) {
	if cp.SendMessageToRemote != nil {
		cp.SendMessageToRemote(message)
	}
	if cp.Conn != nil {
		ba := message.MarshalJSON()
		cp.Conn.Write(ba)
		cp.Conn.Write([]byte{'\n'})
	}
}

func (mux *Mux) RemoveConnection(id string, cp *MsgConnection) {
	mux.mutex.Lock()
	delete(mux.connections, id)
	mux.mutex.Unlock()

	if h, f := mux.handlers["/close"]; f {
		h.HandleMessage(context.Background(), "/close", map[string]string{"id": id}, nil)
	}
	log.Println("/mux/RemoveConnection", id, cp.VIP)
}

func (mc *MsgConnection) Close() {
	mc.gate.RemoveConnection(mc.Name, mc)
}

func (mux *Mux) Id() string {
	mutex.Lock()
	defer mutex.Unlock()
	id++
	return fmt.Sprintf("%d", id)
}

// Message from a remote, will be forwarded to subscribed connections.
func (mux *Mux) OnRemoteMessage(ev *Message, connName string) error {
	// Local handlers first
	if ev.Time == 0 {
		ev.Time = time.Now().Unix()
	}
	if ev.Id == "" {
		ev.Id = mux.Id()
	}
	parts := strings.Split(ev.To, "/")

	if len(parts) < 2 {
		return nil
	}

	to := parts[0]
	if to == mux.Auth.Self() || to == "" {
		mux.HandleMessageForNode(ev)
		log.Println("/mux/OnRemoteMessageLocal", ev.To)
		return nil
	}

	ev.Path = append(ev.Path, mux.Auth.VIP6.String())
	for k, ms := range mux.connections {
		if k == ev.From || k == connName {
			continue
		}
		for _, a := range ev.Path {
			if a == ms.VIP {
				log.Println("/mux/OnRemoteMessageFWD - LOOP ", ev.To, ms.VIP, ms.Name)
				continue
			}
		}
		ms.maybeSend(parts, ev, k)
		log.Println("/mux/OnRemoteMessageFWD", parts[0], ev.To, ms.VIP, ms.Name)
	}
	return nil
}

// Send a message to one or more connections.
func (mux *Mux) SendMsg(ev *Message) error {
	parts := strings.Split(ev.To, "/")

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
	return nil

}

func (ms *MsgConnection) maybeSend(parts []string, ev *Message, k string) {
	// TODO: check the path !
	if parts[0] != "" {
		// TODO: send if the peer ID matches, or if peer has sent a (signed) event message that the node
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

// ProcessMessage parses an incoming message, from a remote connection.
// Message is further processed using one of the methods.
func (mux *Mux) ProcessMessage(line []byte, ctx *auth.ReqContext) *Message {
	if len(line) == 0 {
		return nil
	}
	var ev *Message
	var from string
	if ctx != nil {
		from = ctx.ID()
	}

	if line[0] == '{' {
		ev = ParseJSON(line)

		ev.From = from

		parts := strings.Split(ev.To, "/")
		if len(parts) < 2 {
			log.Println("Invalid To", parts)
			return nil
		}
		top := parts[1]
		ev.Topic = top

		ev.Path = append(ev.Path, from)
	} else {
		parts := strings.Split(string(line), " ")
		meta := map[string]string{}
		if len(parts) > 1 {
			for _, kv := range parts[1:] {
				kvp := strings.SplitN(kv, "=", 2)
				if len(kvp) == 2 {
					meta[kvp[0]] = kvp[1]
				} else {
					meta[kvp[0]] = ""
				}
			}
		}
		ev = NewMessage(parts[0], meta)
	}

	return ev
}

// Messages received from remote.
//
// from is the authenticated VIP of the sender.
// self is my own VIP
//
func (mconn *MsgConnection) HandleMessageStream(cb func(message *Message), br *bufio.Reader, from string) {
	for {
		line, _, err := br.ReadLine()
		if err != nil {
			log.Println("Error reading stream ", mconn.VIP, err)
			break
		}
		//if role == ROLE_GUEST {
		//	continue
		//}
		if len(line) > 0 && line[0] == '{' {
			ev := ParseJSON(line)

			// TODO: if a JWT is present and encrypted or signed binary - use the original from.

			if ev.Time == 0 {
				ev.Time = time.Now().Unix()
			}
			if ev.From == "" {
				ev.From = from
			}

			parts := strings.Split(ev.To, "/")
			if len(parts) < 2 {
				log.Println("Invalid To", parts)
				continue
			}
			top := parts[1]
			ev.Topic = top
			if top == "sub" {
				mconn.SubscriptionsToSend = append(mconn.SubscriptionsToSend, parts[2])
				continue
			}

			// TODO: forwarded 'endpoint' messages, for children and peers

			loop := false
			for _, s := range ev.Path {
				if s == mconn.gate.Auth.Self() {
					loop = true
					break
				}
				if s == ev.From {
					loop = true
					break
				}
			}
			if loop {
				continue
			}
			ev.Path = append(ev.Path, ev.From)
			ev.Connection = mconn

			// For each message
			if cb != nil {
				cb(ev)
			}
			mconn.gate.OnRemoteMessage(ev, mconn.Name)
		}
	}

}
