package ugate

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

type EchoConn struct {

}

func (e EchoConn) Read(b []byte) (n int, err error) {
	panic("implement me")
}

func (e EchoConn) Write(b []byte) (n int, err error) {
	panic("implement me")
}

func (e EchoConn) Close() error {
	panic("implement me")
}

func (e EchoConn) LocalAddr() net.Addr {
	panic("implement me")
}

func (e EchoConn) RemoteAddr() net.Addr {
	panic("implement me")
}

func (e EchoConn) SetDeadline(t time.Time) error {
	panic("implement me")
}

func (e EchoConn) SetReadDeadline(t time.Time) error {
	panic("implement me")
}

func (e EchoConn) SetWriteDeadline(t time.Time) error {
	panic("implement me")
}

type TestDialer struct {

}

func (t TestDialer) DialProxy(ctx context.Context, addr net.Addr, directClientAddr net.Addr, ctype string, meta ...string) (net.Conn, func(client net.Conn) error, error) {
	as := addr.String()
	if strings.HasPrefix(as, "_echo.") {
		return &EchoConn{}, nil, nil
	}
	return nil, nil, nil
}

func (t TestDialer) AcceptForward(in io.ReadCloser, out io.Writer,	remoteIP net.IP, remotePort int) {
}

var td *TestDialer

func TestUGate(t *testing.T) {
	td := &TestDialer{}
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
		Port: 3004,
		Endpoint: td,
	})
	ug.Add(&ListenerConf{
		Port: 3005,
		Remote: "localhost:3000",
	})

}

func BenchmarkUGate(t *testing.B) {

}
