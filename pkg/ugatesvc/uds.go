package ugatesvc

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/costinm/ugate"
)

func (ug *UGate) processUnixConn(bc *ugate.RawConn) error {
	uc, ok := bc.Out.(*net.UnixConn)
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
