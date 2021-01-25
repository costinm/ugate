package ugate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// HTTP handlers for admin, debug and testing

// Control handler, also used for testing
type EchoHandler struct {
}

func (eh *EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("ECHOH ", r)
	w.WriteHeader(200)
	//w.Write([]byte{32})
	w.(http.Flusher).Flush()

	// H2 requests require write to be flushed - buffering happens !
	// Wrap w.Body into Stream which does this automatically
	str := NewStreamRequest(r, w, nil)

	eh.handle(str, false)
}

type StreamInfo struct {
	LocalAddr  net.Addr
	RemoteAddr net.Addr
	Meta       http.Header

	RemoteID string
	ALPN     string

	Dest string
	Type string
}

func (*EchoHandler) handle(str *Stream, serverFirst bool) error {
	d := make([]byte, 2048)

	si := &StreamInfo{
		LocalAddr:  str.LocalAddr(),
		RemoteAddr: str.RemoteAddr(),
		Meta:       str.HTTPRequest().Header,
		RemoteID:   str.RemoteID(),
		Dest:       str.Dest,
		Type:       str.Type,
	}
	if str.TLS != nil {
		si.ALPN = str.TLS.NegotiatedProtocol
	}
	b1, _ := json.Marshal(si)
	b := &bytes.Buffer{}
	b.Write(b1)
	b.Write([]byte{'\n'})

	if serverFirst {
		str.Write(b.Bytes())
	}
	//ac.SetDeadline(time.Now().Add(5 * time.Second))
	n, err := str.Read(d)
	if err != nil {
		return err
	}
	log.Println("ECHO rcv", n)

	if !serverFirst {
		str.Write(b.Bytes())
	}
	str.Write(d[0:n])

	io.Copy(str, str)
	return nil
}

func (eh *EchoHandler) Handle(ac MetaConn) error {
	log.Println("ECHO ", ac.Meta())

	return eh.handle(ac.Meta(), false)
}

// WIP: Adapter to HTTP handlers
// Requests are mapped using IPFS-style, using metadata if available.
type HTTPHandler struct {
}

func (*HTTPHandler) Handle(ac MetaConn) error {
	//u, _ := url.Parse("https://localhost/")
	r := ac.Meta().HTTPRequest()
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

// Debug handlers - on default mux, on 15000

// curl -v http://s6.webinf.info:15000/...
// - /debug/vars
// - /debug/pprof

func (gw *UGate) HttpTCP(w http.ResponseWriter, r *http.Request) {
	gw.m.RLock()
	defer gw.m.RUnlock()
	w.Header().Add("content-type", "application/json")
	err := json.NewEncoder(w).Encode(gw.ActiveTcp)
	if err != nil {
		log.Println("Error encoding ", err)
	}
}

func (gw *UGate) HttpNodesFilter(w http.ResponseWriter, r *http.Request) {
	gw.m.RLock()
	defer gw.m.RUnlock()
	r.ParseForm()
	w.Header().Add("content-type", "application/json")
	t := r.Form.Get("t")
	rec := []*DMNode{}
	t0 := time.Now()
	for _, n := range gw.NodesByID {
		if t != "" {
			if t0.Sub(n.LastSeen) < 6000*time.Millisecond {
				rec = append(rec, n)
			}
		} else {
			rec = append(rec, n)
		}
	}

	je := json.NewEncoder(w)
	je.SetIndent(" ", " ")
	je.Encode(rec)
	return
}

func (gw *UGate) HttpH2R(w http.ResponseWriter, r *http.Request) {
	gw.m.RLock()
	defer gw.m.RUnlock()

	json.NewEncoder(w).Encode(gw.ActiveTcp)
}

// WIP: Istio-style signing
func (gw *UGate) SignCert(w http.ResponseWriter, r *http.Request) {
	// TODO: json and raw proto
	// use a list of 'authorized' OIDC and roots ( starting with loaded istio and k8s pub )
	// get the csr and sign
}
