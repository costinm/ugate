package main

import (
	"context"
	"io"
	"net"
	"net/http"

	"github.com/costinm/ugate"
)


type BasicDialer struct {

}

func (t BasicDialer) DialProxy(ctx context.Context, addr net.Addr, directClientAddr net.Addr, ctype string, meta ...string) (net.Conn, func(client net.Conn) error, error) {
	return nil, nil, nil
}

func (t BasicDialer) AcceptForward(in io.ReadCloser, out io.Writer,	remoteIP net.IP, remotePort int) {
}


func main() {
	ug := ugate.NewGate(&net.Dialer{})

	// Using standard net.Dialer, connect to local iperf3
	ug.Add(&ugate.ListenerConf{
		Port: 3001,
		Remote: "localhost:5201",
	})

	//
	ug.Add(&ugate.ListenerConf{
		Port: 3002,
		Protocol: "sni",
	})
	ug.Add(&ugate.ListenerConf{
		Port: 3003,
		Local: "127.0.0.1:3003",
		Protocol: "socks5",
	})
	// In-process dialer (ssh, etc)
	//ug.Add(&ugate.ListenerConf{
	//	Port: 3004,
	//	Endpoint: td,
	//})

	http.ListenAndServe(":3010", nil)
}

