package ugate

import (
	"expvar"
	"io"
	"net"
	"sync/atomic"
	"time"
)

var (
	streamIds = int32(0)
)


var (
	VarzAccepted = expvar.NewInt("ugate.accepted")
	VarzAcceptErr = expvar.NewInt("ugate.acceptErr")

	// Proxy behavior
	VarzReadFrom = expvar.NewInt("ugate.sReadFrom")
	VarzReadFromC = expvar.NewInt("ugate.cReadFrom")

	VarzSErrRead = expvar.NewInt("ugate.sErrRead")
	VarzSErrWrite = expvar.NewInt("ugate.sErrWrite")
	VarzCErrRead = expvar.NewInt("ugate.cErrRead")
	VarzCErrWrite = expvar.NewInt("ugate.cErrWrite")

	VarzMaxRead = expvar.NewInt("ugate.maxRead")
)

func (s *Stats) Reset() {
	s.Open = time.Now()
	s.LastRead = time.Time{}
	s.LastWrite = time.Time{}

	s.ReadBytes = 0
	s.WriteBytes = 0
	s.ReadPackets = 0
	s.WritePackets = 0

	s.ReadErr = nil
	s.WriteErr = nil
	s.Type = ""

	s.StreamId = atomic.AddInt32(&streamIds, 1)
}

type UGate struct {
	Listeners map[int]*portListener

	Dialer ContextDialer

	// Configurations, keyed by port.
	Conf map[int][]*ListenerConf
}

func NewGate(d ContextDialer) *UGate {
	return &UGate{
		Listeners: map[int]*portListener{},
		Dialer: d,
	}
}

func (ug *UGate) Add(cfg *ListenerConf) (io.Closer, net.Addr, error) {
	l, err := NewListener(ug, cfg)
	if err != nil {
		return nil, nil, err
	}
	return l.Listener, l.Listener.Addr(), nil
}
