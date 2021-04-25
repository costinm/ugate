package ugatesvc

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/costinm/ugate"
)

// HTTP handlers for admin, debug and testing

// Control handler, also used for testing
type EchoHandler struct {
}

func (eh *EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("ECHOH ", r)
	w.WriteHeader(200)
	w.(http.Flusher).Flush()

	// H2 requests require write to be flushed - buffering happens !
	// Wrap w.Body into Stream which does this automatically
	str := ugate.NewStreamRequest(r, w, nil)

	eh.handle(str, false)
}


func (*EchoHandler) handle(str *ugate.Stream, serverFirst bool) error {
	d := make([]byte, 2048)
	si := str.StreamInfo()
	si.RemoteID=   RemoteID(str)
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
	log.Println("ECHO rcv", n, "strid", str.StreamId)

	if !serverFirst {
		str.Write(b.Bytes())
	}
	str.Write(d[0:n])

	io.Copy(str, str)
	if ugate.DebugClose {
		log.Println("ECHO DONE", str.StreamId)
	}
	return nil
}

func (eh *EchoHandler) Handle(ac ugate.MetaConn) error {
	log.Println("ECHOS ", ac.Meta())

	return eh.handle(ac.Meta(), false)
}

//func (h2p *H2P) InitPush(mux http.ServeMux) {
//	mux.Handle("/push/*", h2p)
//}
//
//type H2P struct {
//	// Key is peer ID.
//	// TODO: multiple monitors per peer ?
//	mons map[string]*Pusher
//
//	// Active pushed connections.
//	// Key is peerID / stream ID
//	active map[string]*net.Conn
//}
//
//// Single Pusher - one for each monitor
//type Pusher struct {
//	ch chan string
//}
//
//func NewPusher() *Pusher {
//	return &Pusher{
//		ch: make(chan string, 10),
//	}
//}
//
//func (h2p *H2P) ServeHTTP(w http.ResponseWriter, req *http.Request) {
//	if strings.HasPrefix(req.URL.Path, "/push/mon/") {
//		h2p.HTTPHandlerPushPromise(w, req)
//		return
//	}
//	if strings.HasPrefix(req.URL.Path, "/push/up/") {
//		h2p.HTTPHandlerPost(w, req)
//		return
//	}
//
//	h2p.HTTPHandlerPush(w, req)
//}
//
//// Should be mapped to /push/*
//// If Method is GET ( standard stack ), only send the content of the con, expect
//// a separate connection for the POST side.
//func (h2p *H2P) HTTPHandlerPush(w http.ResponseWriter, req *http.Request) {
//	w.WriteHeader(200)
//	w.Write([]byte{1})
//	io.Copy(w, req.Body)
//}
//
//func (h2p *H2P) HTTPHandlerPost(w http.ResponseWriter, req *http.Request) {
//	w.WriteHeader(200)
//	w.Write([]byte{1})
//	io.Copy(w, req.Body)
//}
//
//// Hanging - will send push frames, corresponding HTTPHandlerPush will be called to service
//// the stream. When used with the 'standard' h2 stack only GET is supported.
//func (h2p *H2P) HTTPHandlerPushPromise(w http.ResponseWriter, req *http.Request) {
//	w.WriteHeader(200)
//
//	p := NewPusher()
//
//	ctx := req.Context()
//	for {
//		select {
//		case ev := <-p.ch:
//			opt := &http.PushOptions{
//				Header: http.Header{
//					"User-Agent": {"foo"},
//				},
//			}
//			// This will result in a separate handler to get the message
//			if err := w.(http.Pusher).Push("/push/"+ev, opt); err != nil {
//				fmt.Println("error pushing", err)
//				return
//			}
//		case <-ctx.Done():
//			return
//		}
//	}
//}

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
	rec := []*ugate.DMNode{}
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

//// WIP: Istio-style signing
//func (gw *UGate) SignCert(w http.ResponseWriter, r *http.Request) {
//	// TODO: json and raw proto
//	// use a list of 'authorized' OIDC and roots ( starting with loaded istio and k8s pub )
//	// get the csr and sign
//}
//

// HandleID is the first request in a MUX connection.
//
// If the request is authenticated, we'll track the node.
// For QUIC, mTLS handshake completes after 0RTT requests are received, so JWT is
// needed.
func (gw *UGate) HandleID(w http.ResponseWriter, r *http.Request) {
	f := r.Header.Get("from")
	if f == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	//n := gw.GetOrAddNode(f)

	w.WriteHeader(200)
	w.Write([]byte(gw.Auth.ID))
}

// HandleTCPPRoxy is called for CONNECT and /dm/ADDRESS
// TODO: also handle /ipfs/... for compat.
func (gw *UGate) HandleTCPProxy(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.RequestURI, "/")
	//
	//r1 := CreateUpstreamRequest(w, r)
	//r1.Host = parts[2]
	//r1.URL.Scheme = "https"
	//r1.URL.Host = r1.Host
	//r1.URL.Path = "/" + strings.Join(parts[3:], "/")

	str := ugate.NewStreamRequest(r, w, nil)
	str.Dest = parts[2]
	str.PostDialHandler = func(conn net.Conn, err error) {
		if err != nil {
			w.Header().Add("Error", err.Error())
			w.WriteHeader(500)
			w.(http.Flusher).Flush()
			return
		}
		w.Header().Set("Trailer", "X-Close")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
	}
	str.Dest = parts[2]

	gw.HandleVirtualIN(str)

}
