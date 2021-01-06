package ugate

import (
	"errors"
	"fmt"
	"net"
	"syscall"
)

func (ug *UGate) processUnixConn(bc *RawConn) error {
	uc, ok := bc.ServerOut.(*net.UnixConn)
	if !ok {
		return errors.New("Unexpected con")
	}
	enableUnixCredentials(uc)

	return nil
}

// Enable reception of PID/UID/GID
func enableUnixCredentials(conn *net.UnixConn) error {
	viaf, err := conn.File()
	if err != nil {
		return fmt.Errorf("UDS convert connection to file descriptor: %v", err)
	}
	err = syscall.SetsockoptInt(int(viaf.Fd()), syscall.SOL_SOCKET, syscall.SO_PASSCRED, 1)
	return err
}
