package ssh

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
)

// Based on okteto code: https://raw.githubusercontent.com/okteto/remote/main/pkg/ssh/ssh.go
// Removed deps on logger, integrated with ugate.

// Handles PTY/noPTY shell sessions and sftp.

var (
	idleTimeout = 60 * time.Second

	// ErrEOF is the error when the terminal exits
	ErrEOF = errors.New("EOF")
)


func getExitStatusFromError(err error) int {
	if err == nil {
		return 0
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return 1
	}

	waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		if exitErr.Success() {
			return 0
		}

		return 1
	}

	return waitStatus.ExitStatus()
}

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

func handlePTY(cmd *exec.Cmd, s ssh.Session, ptyReq ssh.Pty, winCh <-chan ssh.Window) error {
	if len(ptyReq.Term) > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
	}

	f, err := pty.Start(cmd)
	if err != nil {
		log.Println("failed to start pty session", err)
		return err
	}

	go func() {
		for win := range winCh {
			setWinsize(f, win.Width, win.Height)
		}
	}()

	go func() {
		io.Copy(f, s) // stdin
	}()

	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		io.Copy(s, f) // stdout
	}()

	if err := cmd.Wait(); err != nil {
		log.Println("pty command failed while waiting", err)
		return err
	}

	select {
	case <-waitCh:
		log.Println("stdout finished")
	case <-time.NewTicker(1 * time.Second).C:
		log.Println("stdout didn't finish after 1s")
	}

	return nil
}

func sendErrAndExit(s ssh.Session, err error) {
	msg := strings.TrimPrefix(err.Error(), "exec: ")
	if _, err := s.Stderr().Write([]byte(msg)); err != nil {
		log.Println("failed to write error back to session", err)
	}

	if err := s.Exit(getExitStatusFromError(err)); err != nil {
		log.Println(err, "pty session failed to exit")
	}
}

func handleNoTTY(cmd *exec.Cmd, s ssh.Session) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println(err, "couldn't get StdoutPipe")
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Println(err, "couldn't get StderrPipe")
		return err
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Println(err, "couldn't get StdinPipe")
		return err
	}

	if err = cmd.Start(); err != nil {
		log.Println(err, "couldn't start command '%s'", cmd.String())
		return err
	}

	go func() {
		defer stdin.Close()
		if _, err := io.Copy(stdin, s); err != nil {
			log.Println(err, "failed to write session to stdin.")
		}
	}()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(s, stdout); err != nil {
			log.Println(err, "failed to write stdout to session.")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(s.Stderr(), stderr); err != nil {
			log.Println(err, "failed to write stderr to session.")
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		log.Println(err, "command failed while waiting")
		return err
	}

	return nil
}

func (srv *Server) connectionHandler(s ssh.Session) {
	defer func() {
		s.Close()
		log.Println("session closed")
	}()

	log.Printf("starting ssh session with command '%+v'", s.RawCommand())

	cmd := srv.buildCmd(s)

	if ssh.AgentRequested(s) {
		log.Println("agent requested")
		l, err := ssh.NewAgentListener()
		if err != nil {
			log.Println("failed to start agent", err)
			return
		}

		defer l.Close()
		go ssh.ForwardAgentConnections(l, s)
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", "SSH_AUTH_SOCK", l.Addr().String()))
	}

	ptyReq, winCh, isPty := s.Pty()
	if isPty {
		log.Println("handling PTY session")
		if err := handlePTY(cmd, s, ptyReq, winCh); err != nil {
			sendErrAndExit(s, err)
			return
		}

		s.Exit(0)
		return
	}

	log.Println("handling non PTY session")
	if err := handleNoTTY(cmd, s); err != nil {
		sendErrAndExit(s, err)
		return
	}

	s.Exit(0)
}


func (srv *Server) authorize(ctx ssh.Context, key ssh.PublicKey) bool {
	for _, k := range srv.AuthorizedKeys {
		if ssh.KeysEqual(key, k) {
			return true
		}
	}

	log.Println("access denied")
	return false
}


func sftpHandler(sess ssh.Session) {
	debugStream := ioutil.Discard
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(debugStream),
	}
	server, err := sftp.NewServer(
		sess,
		serverOptions...,
	)
	if err != nil {
		log.Printf("sftp server init error: %s\n", err)
		return
	}
	if err := server.Serve(); err == io.EOF {
		server.Close()
		log.Println("sftp client exited session.")
	} else if err != nil {
		log.Println("sftp server completed with error:", err)
	}
}

func (srv Server) buildCmd(s ssh.Session) *exec.Cmd {
	var cmd *exec.Cmd

	if len(s.RawCommand()) == 0 {
		cmd = exec.Command(srv.Shell)
	} else {
		args := []string{"-c", s.RawCommand()}
		cmd = exec.Command(srv.Shell, args...)
	}

	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, s.Environ()...)

	fmt.Println(cmd.String())
	return cmd
}
