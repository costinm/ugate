package ugate

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

// HTTP handlers for admin, debug and testing

type EchoHandler struct {
}

func (*EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("ECHOH ", r)

	d := make([]byte, 2048)
	//fmt.Fprintf(ac, "%v\n", ac.Meta())
	w.WriteHeader(200)
	w.Write([]byte{32})
	w.(http.Flusher).Flush()

	n, err := r.Body.Read(d)
	if err != nil {
		return
	}
	log.Println("ECHO rcv", n )
	b := &bytes.Buffer{}
	r.Write(b)
	fmt.Fprintf(b, "%s %v\n", r.RemoteAddr, r.TLS)
	b.Write(d[0:n])

	w.Write(b.Bytes())
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	io.Copy(w, r.Body)
}

func (*EchoHandler) Handle(ac MetaConn) error {
	log.Println("ECHO ", ac.Meta())
	//ac.Write([]byte{1})
	d := make([]byte, 2048)
	//fmt.Fprintf(ac, "%v\n", ac.Meta())
	//ac.SetDeadline(time.Now().Add(5 * time.Second))
	n, err := ac.Read(d)
	if err != nil {
		return err
	}
	log.Println("ECHO rcv", n )

	b := &bytes.Buffer{}
	ac.Meta().Request.Write(b)
	fmt.Fprintf(b, "%v\n", ac.Meta().Request.TLS)
	b.Write(d[0:n])

	ac.Write(b.Bytes())

	io.Copy(ac, ac)
	return nil
}

// WIP: Adapter to HTTP handlers
// Requests are mapped using IPFS-style, using metadata if available.
type HTTPHandler struct {
}

func (*HTTPHandler) Handle(ac MetaConn) error {
	//u, _ := url.Parse("https://localhost/")
	r := ac.Meta().Request
	w := ac.(http.ResponseWriter)
	http.DefaultServeMux.ServeHTTP(w, r)
	return nil
}

func (h2p *H2P) InitPush(mux http.ServeMux) {
	mux.Handle("/push/*", h2p)
}

type H2P struct {
	// Key is peer ID.
	// TODO: multiple monitors per peer ?
	mons map[string]*Pusher

	// Active pushed connections.
	// Key is peerID / stream ID
	active map[string]*net.Conn

}

// Single Pusher - one for each monitor
type Pusher struct {
	ch chan string
}

func NewPusher() *Pusher {
	return &Pusher{
		ch: make(chan string, 10),
	}
}

func (h2p *H2P) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if strings.HasPrefix(req.URL.Path, "/push/mon/") {
		h2p.HTTPHandlerPushPromise(w, req)
		return
	}
	if strings.HasPrefix(req.URL.Path, "/push/up/") {
		h2p.HTTPHandlerPost(w, req)
		return
	}

	h2p.HTTPHandlerPush(w, req)
}

// Should be mapped to /push/*
// If Method is GET ( standard stack ), only send the content of the con, expect
// a separate connection for the POST side.
func (h2p *H2P) HTTPHandlerPush(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte{1})
	io.Copy(w, req.Body)
}

func (h2p *H2P) HTTPHandlerPost(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte{1})
	io.Copy(w, req.Body)
}

// Hanging - will send push frames, corresponding HTTPHandlerPush will be called to service
// the stream. When used with the 'standard' h2 stack only GET is supported.
func (h2p *H2P) HTTPHandlerPushPromise(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)

	p := NewPusher()

	ctx := req.Context()
	for {
		select {
		case ev := <-p.ch:
			opt := &http.PushOptions{
				Header: http.Header{
					"User-Agent": {"foo"},
				},
			}
			// This will result in a separate handler to get the message
			if err := w.(http.Pusher).Push("/push/"+ev, opt); err != nil {
				fmt.Println("error pushing", err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

