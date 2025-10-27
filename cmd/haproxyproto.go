package cmd

import (
	"context"
)

import "github.com/pires/go-proxyproto"

//
// Istio uses pires/go-proxyproto
// Also armon/proxyproto - text only, wraps conn and uses a bufio without optimization.

type HAProxy struct {
	l *proxyproto.Listener
}

func (h *HAProxy) Start(ctx context.Context) error {
	//proxyproto.NewConn()
	l := &proxyproto.Listener{}
	h.l = l

	go func() {
		con, err := l.Accept()
		if err != nil {
			return
		}
		con.Close()
	}()

	return nil
}
