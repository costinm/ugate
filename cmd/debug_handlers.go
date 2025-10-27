package cmd

import (
	"sync"
)

// HTTP handlers for admin, debug and testing


// Debug handlers - on default mux, on 15000

// curl -v http://s6.webinf.info:15000/...
// - /debug/vars
// - /debug/pprof

type UGateHandlers struct {
	m sync.RWMutex
}

