package ugated

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/costinm/ugate"
)

// HTTP handlers for admin, debug and testing


// Debug handlers - on default mux, on 15000

// curl -v http://s6.webinf.info:15000/...
// - /debug/vars
// - /debug/pprof

type UGateHandlers struct {
	m sync.RWMutex
	UGate *ugate.UGate
}

func InitHandlers(ug *ugate.UGate) {
	ugh := &UGateHandlers{UGate: ug}
	ug.Mux.HandleFunc("/dmesh/tcpa", ugh.HttpTCP)
	ug.Mux.HandleFunc("/dmesh/rd", ugh.HttpNodesFilter)

	//ug.H2Transport.HandleFunc("/h2r/", ugh.HandleH2R)

	ug.Mux.HandleFunc("/_dm/", ugh.HandleID)

}

func (gw *UGateHandlers) HttpTCP(w http.ResponseWriter, r *http.Request) {
	gw.m.RLock()
	defer gw.m.RUnlock()
	w.Header().Add("content-type", "application/json")
	err := json.NewEncoder(w).Encode(gw.UGate.ActiveTcp)
	if err != nil {
		log.Println("Error encoding ", err)
	}
}

// HttpNodesFilter returns the list of directly connected nodes.
//
// Optional 't' parameter is a timestamp used to filter recently seen nodes.
// Uses NodeByID table.
func (gw *UGateHandlers) HttpNodesFilter(w http.ResponseWriter, r *http.Request) {
	gw.m.RLock()
	defer gw.m.RUnlock()
	r.ParseForm()
	w.Header().Add("content-type", "application/json")
	t := r.Form.Get("t")
	rec := []*ugate.MeshCluster{}
	t0 := time.Now()
	for _, n := range gw.UGate.Clusters {
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

func (gw *UGateHandlers) HttpH2R(w http.ResponseWriter, r *http.Request) {
	gw.m.RLock()
	defer gw.m.RUnlock()

	json.NewEncoder(w).Encode(gw.UGate.ActiveTcp)
}

// HandleID is the first request in a MUX connection.
//
// If the request is authenticated, we'll track the node.
// For QUIC, mTLS handshake completes after 0RTT requests are received, so JWT is
// needed.
func (gw *UGateHandlers) HandleID(w http.ResponseWriter, r *http.Request) {
	f := r.Header.Get("from")
	if f == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	//n := gw.GetOrAddNode(f)

	w.WriteHeader(200)
	w.Write([]byte(gw.UGate.Auth.ID))
}
