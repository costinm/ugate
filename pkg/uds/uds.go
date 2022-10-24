package uds

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"syscall"

	"github.com/costinm/hbone/nio"
	msgs "github.com/costinm/ugate/webpush"
)

// uds provides helpers for passing credentials and files over UDS streams, and basic
// message passing.
//
// Typical use case is to have a process with elevated priviledges open files or ports,
// and pass them to lower-priv processes.
//
// It also has an android variant, used to send TUN id and messages with a native application.
//
// Protocol is based on gRPC bi-directional streaming, and should be usable with a proxy that
// forwards the raw stream body of a H2 gRPC connection.
// That means messages start with a 5-byte header:
//
// TYPE (1byte): 0 for protobuf content, 2
// LEN (4 byte): len of the data, not including the 5-byte header.
//
//

// UdsConn represents a client or server connection. This is not thread safe, should be used from a single
// receiver routine. Implements msgs.Framer and has a Send function.
type UdsConn struct {
	msgs.MsgConnection

	con *net.UnixConn

	Handler msgs.MessageHandler

	Reader *bufio.Reader
	oob    []byte
	buffer []byte

	// off in buffer where next read will happen. End of existing data
	off          int
	leftoverSize int

	// Name of the UDS file. The unix addr will be under net=unix, as @Name
	Name string

	mux *msgs.Mux
	// Received file descriptors.
	Files  []*os.File
	closed bool

	Pid      int32
	Uid, Gid uint32
	initial  map[string]string
}

type UdsServer struct {
	listener *net.UnixListener

	// Name of the server.
	Name string

	//Handlers map[string]MessageHandler

	Gid int

	mux *msgs.Mux

	mutex sync.RWMutex
}

var (
	Debug = false
)

// Create a UDS server listening on 'name'.
// go Dial()  must be called to accept.
//
// Messages posted on the mux will be sent to all clients (that subscribe - TODO).
// Messages received from clients will be passed to mux handlers.
func NewServer(name string, mux *msgs.Mux) (*UdsServer, error) {
	us, err := net.ListenUnix("unix", &net.UnixAddr{Name: "@" + name, Net: "unix"})
	if err != nil {
		return nil, err
	}
	return &UdsServer{Name: name,
		listener: us,
		Gid:      -1,
		mux:      mux,
		//Handlers: map[string]MessageHandler{},
	}, nil
}

// Dial a single client connection and handshake.
func Dial(ifname string, mux *msgs.Mux, initial map[string]string) (*UdsConn, error) {
	uds := &UdsConn{
		oob:     make([]byte, syscall.CmsgSpace(256)),
		buffer:  make([]byte, 32*1024),
		Name:    ifname,
		initial: initial,
		mux:     mux,
	}
	uds.SendMessageToRemote = uds.SendMessage
	err := uds.Redial()

	return uds, err
}

func (uc *UdsConn) Redial() error {
	ucon, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: "@" + uc.Name, Net: "unix"})
	if err != nil {
		return err
	}
	uc.con = ucon
	uc.Reader = bufio.NewReader(ucon)

	err = uc.handshakeClient()

	return err
}

func (uds *UdsServer) Close() {
	uds.listener.Close()
}

// accept a single UdsConn, as a server.
func (uds *UdsServer) accept() (*UdsConn, error) {
	inS, err := uds.listener.AcceptUnix()
	if err != nil {
		fmt.Println("Error accepting ", uds.Name, err)
		return nil, err
	}
	err = enableUnixCredentials(inS)
	if err != nil {
		log.Println("Error enabling unix creds ", err)
	}
	uc := &UdsConn{
		con:    inS,
		oob:    make([]byte, syscall.CmsgSpace(256)),
		buffer: make([]byte, 32*1024),
		Name:   uds.Name,
		mux:    uds.mux,
		Reader: bufio.NewReader(inS),
	}
	uc.SendMessageToRemote = uc.SendMessage
	return uc, nil
}

func (uds *UdsServer) Start() error {
	// TODO: stop and it's channel
	for {
		inS, err := uds.accept()
		if err != nil {
			fmt.Println("Error accepting ", uds.Name, err)
			return err
		}
		go uds.serverStream(inS)
	}
	return nil
}

var cnt = 0

// Called after connecting to the remote UDS. Must send something, so credentials are passed.
func (conn *UdsConn) serverHandshake(uds *UdsServer) error {
	//data := make([]byte, 4096)

	// Server side

	// First message from client should include unix credentials
	// Should also include the identity of the node !
	_, data, err := conn.nextMessage()
	//ndata, pid, uid, gid, err := conn.GetUnixCreds(data)
	if err != nil {
		return errors.New(fmt.Sprint("Failed to read unix creds ", err))
	}

	if uds.Gid != -1 && uint32(uds.Gid) != conn.Gid {
		return errors.New(fmt.Sprintln("Invalid GID, expecting ", uds.Gid, conn.Pid, conn.Uid, conn.Gid))
	}

	conn.Name = fmt.Sprintf("udss:%d:%d:%d", conn.Uid, conn.Pid, cnt)
	cnt++

	conn.SendMessageDirect("ok", nil, nil)

	// First message: :open
	log.Println("DMesh UDS connection process ", conn.Pid, conn.Uid, conn.Gid, string(data))

	log.Println("UDS: Connection handshake")
	return nil
}

// TODO: include 'id' ( public key ) and signature
func (conn *UdsConn) handshakeClient() error {

	// client side
	conn.SendMessageDirect(":open", conn.initial, nil)

	// read server response
	mtype, data, err := conn.nextMessage()
	if err != nil {
		// VPN will remain open - for the next connection
		log.Println("Error reading initial message, close UDS ", mtype, data, err)
		conn.con.Close()
		return err
	}
	cmd, meta, payload, _ := ParseMessage(data, mtype)

	log.Println("UDS: client connection handshake", cmd, meta, payload)
	return nil
}

func (uds *UdsConn) Close() {
	uds.closed = true
	uds.con.Close()
}

func (uds *UdsServer) serverStream(conn *UdsConn) {

	// TODO: add connection WorkloadID
	conn.serverHandshake(uds)

	conn.streamCommon()
}

func (conn *UdsConn) streamCommon() {
	conn.SubscriptionsToSend = []string{"I", "ble", "bt", "N", "wifi", "net"}

	conn.mux.AddConnection(conn.Name, &conn.MsgConnection)

	defer func() {
		log.Println("Connection closed ", conn.Name)
		conn.con.Close()
		conn.mux.RemoveConnection(conn.Name, &conn.MsgConnection)
	}()

	for !conn.closed {
		mtype, data, err := conn.nextMessage()
		if err != nil {
			// VPN will remain open - for the next connection
			log.Println("Error reading, close UDS ", conn.Name, err)
			return
		}

		cmd, meta, payload, _ := ParseMessage(data, mtype)

		if Debug {
			log.Println("UDS IN: ", cmd, meta, string(payload))
		}

		msg := &msgs.Message{
			MessageData: msgs.MessageData{
				To:   cmd,
				Meta: meta,
				From: conn.Name,
			},
			Connection: &conn.MsgConnection,
		}

		// Creates a copy of payload, the buffer will be reused on next message.
		// TODO: this can be avoided if OnMessage doesn't create a go-routine and is making its own copy.
		if payload != nil && len(payload) > 0 {
			msg.Data = string(payload)
		}

		if conn.Handler != nil {
			conn.Handler.HandleMessage(context.Background(),
				cmd, meta, payload)
		}

		// Don't forward messages from the UDS.
		conn.mux.HandleMessageForNode(msg)
	}
}

// Handle the client messages, after dialing to a server.
func (conn *UdsConn) HandleStream() {
	conn.streamCommon()
}

func packMeta(meta map[string]string) []byte {
	if meta == nil || len(meta) == 0 {
		return []byte{'\n'}
	}
	buf := bytes.Buffer{}
	for k, v := range meta {
		buf.Write([]byte(k))
		buf.Write([]byte{':'})
		buf.Write([]byte(v))
		buf.Write([]byte{'\n'})
	}
	buf.Write([]byte{'\n'})

	return buf.Bytes()
}

// WriteFD will write data as well as File
func (uds *UdsConn) WriteFD(data []byte, file *os.File) (int, error) {
	rights := syscall.UnixRights(int(file.Fd()))
	n, _, err := uds.con.WriteMsgUnix(data, rights, nil)
	return n, err
}

func (uds *UdsConn) WriteFDs(data []byte, file []*os.File) (int, error) {
	fd := make([]int, len(file))
	for i, f := range file {
		fd[i] = int(f.Fd())
	}
	rights := syscall.UnixRights(fd...)
	n, _, err := uds.con.WriteMsgUnix(data, rights, nil)
	return n, err
}

func (uds *UdsConn) Read(out []byte) (int, error) {
	nresnow, oobn, _, _, err := uds.con.ReadMsgUnix(out, uds.oob)

	if Debug {
		log.Println("uds: read()", nresnow, oobn)
	}
	if err != nil {
		log.Println("UDS ERR", err, nresnow)
		return nresnow, err
	} else if nresnow == 0 && oobn == 0 {
		log.Println("UDS EOF", oobn, nresnow)
		return nresnow, io.EOF
	}

	if oobn > 0 {
		msgs, err := syscall.ParseSocketControlMessage(uds.oob[:oobn])
		if err != nil {
			return nresnow, fmt.Errorf("parse control message: %v", err)
		}

		//log.Println("UDS OOB ", oobn)
		for i := 0; i < len(msgs); i++ {
			if msgs[i].Header.Type == syscall.SCM_CREDENTIALS {
				if uds.Pid != 0 {
					continue
				}
				ncred, err1 := syscall.ParseUnixCredentials(&msgs[0])
				if err1 != nil {
					log.Println("Error parsing unix creds ", err1)
					err = err1
				} else {
					uds.Pid = ncred.Pid
					uds.Uid = ncred.Uid
					uds.Gid = ncred.Gid
				}
				continue
			}

			// File (SCM_RIGHTS)
			fds, err := syscall.ParseUnixRights(&msgs[i])

			if Debug {
				log.Println("UDS   OOB FILES ", fds, err)
			}
			if err != nil {
				// EINVAL is ok
				continue
			} else {
				for j := 0; j < len(fds); j++ {
					if fds[j] != 0 {
						syscall.SetNonblock(int(fds[j]), false)
						f := os.NewFile(uintptr(fds[j]), uds.Name+"-socket-"+strconv.Itoa(j))
						if Debug {
							log.Print("UDS Received OOB ", fds)
						}
						uds.Files = append(uds.Files, f)
					}
				}
			}
		}
	}
	return nresnow, nil
}

// Return a Size-prefixed slice containing the next message.
// The content must be handled before next call - will be replaced.
// Format: same as gRPC stream:
// - type 1B
// - len 4B
// - content
func (uds *UdsConn) nextMessage() (int, []byte, error) {
	endData := 0

	// unprocessed bytes from previous message are left in buf, after 'off' (end of previous message)
	if uds.leftoverSize > 0 && uds.off > 0 {
		if Debug {
			log.Println("uds: move leftover ", uds.leftoverSize, uds.off)
		}
		copy(uds.buffer[0:], uds.buffer[uds.off:uds.off+uds.leftoverSize])

		endData = uds.leftoverSize
		uds.leftoverSize = 0
	}
	for {
		if endData >= 5 {
			len1 := int(binary.LittleEndian.Uint32(uds.buffer[1:]))

			if Debug {
				log.Println("UDS: Read packet head ", len1, endData)
			}
			if len1 > len(uds.buffer) || len1 == 0 {
				log.Println("UDS: Invalid len ", len1)
				return 0, nil, errors.New("Message too large")
			}
			if endData < len1+5 {
				// More data needed, will continue to reading statement
			} else {
				if Debug {
					log.Println("uds: full packet availabe ", len1, endData)
				}
				if endData > len1+5 {
					uds.off = len1 + 5
					uds.leftoverSize = endData - len1 - 5
					if true || Debug {
						log.Println("uds: data left in buffer ", endData, len1, uds.leftoverSize)
					}
					return int(uds.buffer[0]), uds.buffer[5:uds.off], nil
				} else {
					uds.off = 0
					uds.leftoverSize = 0
					return int(uds.buffer[0]), uds.buffer[5 : len1+5], nil
				}
			}
		}

		nresnow, err := uds.Read(uds.buffer[endData:])

		if err != nil {
			log.Println("UDS ERR", err, endData)
			return 0, nil, err
		} else if nresnow == 0 {
			log.Println("UDS EOF", nresnow)
			return 0, nil, io.EOF
		}

		endData += nresnow
	}
}

// Distribute the message to all UDS connections
// Implements the Transport interface.
func (uds *UdsConn) SendMessage(m *msgs.Message) error {
	// TODO: may need go routines to avoid blocking
	_, err := SendFrameLenBinary(uds.con, []byte(m.To), []byte{'\n'}, packMeta(m.Meta), m.Binary())
	return err
}

// Send a message over a UDS connection. Framed with a length prefix.
func (uds *UdsConn) SendMessageDirect(cmd string, meta map[string]string, data []byte) error {
	if uds == nil {
		return nil
	}
	_, err := SendFrameLenBinary(uds.con, []byte(cmd), []byte{'\n'}, packMeta(meta), data)
	return err
}

// File returns a received file descriptor, or nil
func (uds *UdsConn) File() *os.File {
	if len(uds.Files) == 0 {
		return nil
	}
	fd := uds.Files[0]
	uds.Files[0] = nil
	nf := []*os.File{}
	uds.Files = append(nf, uds.Files[1:]...)
	return fd
}

func processUnixConn(bc *nio.Stream) error {
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
