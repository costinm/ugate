package ugate

import (
	"net"
	"testing"
)


func TestUGate(t *testing.T) {
	td := &net.Dialer{}
	ug := NewGate(td)

	ug.Add(&ListenerConf{
		Port: 3000,
		Protocol: "echo",
	})
	ug.Add(&ListenerConf{
		Port: 3001,
		Protocol: "static",
	})
	ug.Add(&ListenerConf{
		Port: 3002,
		Protocol: "delay",
	})
	ug.Add(&ListenerConf{
		Port: 3006,
		Protocol: "sni",
	})
	ug.Add(&ListenerConf{
		Port: 3003,
		Local: "127.0.0.1:3003",
		Protocol: "socks5",
	})
	// In-process dialer (ssh, etc)
	ug.Add(&ListenerConf{
		Port:   3004,
		Dialer: td,
	})
	ug.Add(&ListenerConf{
		Port: 3005,
		Remote: "localhost:3000",
	})

	ug.Add(&ListenerConf{
		Port: 3006,
		Handler: &EchoHandler{},
	})

}

func BenchmarkUGate(t *testing.B) {

}
