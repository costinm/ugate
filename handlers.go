package ugate

import "io"

// For debug and testing

type EchoHandler struct {
}

func (*EchoHandler) Handle(ac *BufferedConn) error {
	io.Copy(ac, ac)
	ac.Close()
	return nil
}
