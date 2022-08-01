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

var DebugEcho = false

func (eh *EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if DebugEcho {
		log.Println("ECHOH ", r)
	}
	w.WriteHeader(200)
	w.(http.Flusher).Flush()

	// H2 requests require write to be flushed - buffering happens !
	// Wrap w.Body into Stream which does this automatically
	str := ugate.NewStreamRequest(r, w, nil)

	eh.handle(str, false)
}

// StreamInfo tracks informations about one stream.
type StreamInfo struct {
	LocalAddr  net.Addr
	RemoteAddr net.Addr

	Meta http.Header

	RemoteID string

	ALPN string

	Dest string

	Type string
}

func GetStreamInfo(str *ugate.Stream) *StreamInfo {
	si := &StreamInfo{
		LocalAddr:  str.LocalAddr(),
		RemoteAddr: str.RemoteAddr(),
		Dest:       str.Dest,
		Type:       str.Type,
	}
	if str.Request != nil {
		si.Meta = str.Request.Header
	}
	if str.TLS != nil {
		si.ALPN = str.TLS.NegotiatedProtocol
	}

	return si
}

func (*EchoHandler) handle(str *ugate.Stream, serverFirst bool) error {
	d := make([]byte, 2048)
	si := GetStreamInfo(str)
	si.RemoteID = RemoteID(str)
	b1, _ := json.Marshal(si)
	b := &bytes.Buffer{}
	b.Write(b1)
	b.Write([]byte{'\n'})

	if serverFirst {
		str.Write(b.Bytes())
	}
	//ac.SetDeadline(time.Now().StartListener(5 * time.Second))
	n, err := str.Read(d)
	if err != nil {
		return err
	}
	if DebugEcho {
		log.Println("ECHO rcv", n, "strid", str.StreamId)
	}
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
func (eh *EchoHandler) String() string {
	return "Echo"
}
func (eh *EchoHandler) Handle(ac *ugate.Stream) error {
	if DebugEcho {
		log.Println("ECHOS ", ac)
	}
	return eh.handle(ac, false)
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
//	active map[string]*net.Stream
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

// HttpNodesFilter returns the list of directly connected nodes.
//
// Optional 't' parameter is a timestamp used to filter recently seen nodes.
// Uses NodeByID table.
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

	// Create a stream, used for proxy with caching.
	str := ugate.NewStreamRequest(r, w, nil)

	if r.Method == "CONNECT" {
		str.Dest = r.Host
	} else {
		parts := strings.Split(r.RequestURI, "/")
		str.Dest = parts[2]
	}

	str.PostDialHandler = func(conn net.Conn, err error) {
		if err != nil {
			w.Header().Add("Error", err.Error())
			w.WriteHeader(500)
			w.(http.Flusher).Flush()
			return
		}
		//w.Header().Set("Trailer", "X-Close")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
	}
	defer func() {
		// Handler is done - even if it didn't call close, prevent calling it again.
		if ugate.DebugClose {
			log.Println("HTTP.Close - handler done, no close ", str.Dest)
		}
		str.ServerClose = true
	}()

	// Treat it as regular stream forwarding
	gw.HandleVirtualIN(str)

	if ugate.DebugClose {
		log.Println("Handler closed for ", r.RequestURI)
	}

}

// Inbound TLS over H2C.
// Used in KNative or similar environmnets that get a H2 POST or CONNECT
// Dest is local.
func (gw *UGate) HandleTLSoverH2(w http.ResponseWriter, r *http.Request) {
	// Create a stream, used for proxy with caching.
	str := ugate.NewStreamRequest(r, w, nil)

	parts := strings.Split(r.RequestURI, "/")
	str.Dest = parts[2]

	str.PostDialHandler = func(conn net.Conn, err error) {
		if err != nil {
			w.Header().Add("Error", err.Error())
			w.WriteHeader(500)
			w.(http.Flusher).Flush()
			return
		}
		//w.Header().Set("Trailer", "X-Close")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
	}
	defer func() {
		// Handler is done - even if it didn't call close, prevent calling it again.
		if ugate.DebugClose {
			log.Println("HTTP.Close - handler done, no close ", str.Dest)
		}
		str.ServerClose = true
	}()

	tlsCfg := gw.TLSConfig
	tc, err := gw.NewTLSConnIn(str.Context(), nil, str, tlsCfg)
	if err != nil {
		str.ReadErr = err
		log.Println("TLS: ", str.RemoteAddr(), str.Dest, str.Route, err)
		return
	}

	// Treat it as regular stream forwarding
	gw.HandleVirtualIN(tc)

	if ugate.DebugClose {
		log.Println("Handler closed for ", r.RequestURI)
	}

}
