package iptables

import (
	"errors"
	"log"
	"net"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

//
func IptablesCapture(ug *ugatesvc.UGate, addr string, in bool) error {
	// For http proxy we need a dedicated plain HTTP port
	nl, err := net.Listen("tcp", addr)
	if err != nil {
		log.Println("Failed to listen", err)
		return err
	}
	for {
		remoteConn, err := nl.Accept()
		ugate.VarzAccepted.Add(1)
		if ne, ok := err.(net.Error); ok {
			ugate.VarzAcceptErr.Add(1)
			if ne.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		if err != nil {
			log.Println("Accept error, closing iptables listener ", err)
			return err
		}
		go handleAcceptedConn(ug, remoteConn, in)
	}

	return nil
}

// Mirroring handleAcceptedConn in UGate
func handleAcceptedConn(ug *ugatesvc.UGate, acceptedCon net.Conn, in bool) {
	bconn := ugate.GetConn(acceptedCon)
	ug.OnStream(bconn.Meta())
	defer ug.OnStreamDone(bconn)
	str := bconn.Meta()

	//case ugate.ProtoIPTablesIn:
	//	// iptables is replacing the conn - process before creating the buffer
	// DestAddr is also set as a sideeffect
	str.Dest, str.ReadErr = SniffIptables(str)

	if str.ReadErr != nil {
		return
	}

	cfg := ug.FindCfgIptablesIn(bconn)
	if cfg.ForwardTo != "" {
		str.Dest = cfg.ForwardTo
	}
	str.Listener = cfg
	str.Type = cfg.Protocol

	if !in {
		str.Egress = true
	}

	str.ReadErr = ug.HandleStream(str)
}

// Status:
// - TCP capture with redirect works
// - capture with TPROXY is not possible - TPROXY is on the PREROUTING chain only,
// not touched by output packets.
//   https://upload.wikimedia.org/wikipedia/commons/3/37/Netfilter-packet-flow.svg
//
// It works great for transparent proxy in a gateway/router - however same can be also done using the TUN
// and routing to the TUN using iptables or other means.

// Using: https://github.com/Snawoot/transocks/blob/v1.0.0/original_dst_linux.go
// ServeConn is used to serve a single TCP UdpNat.
// See https://github.com/cybozu-go/transocks
// https://github.com/ryanchapman/go-any-proxy/blob/master/any_proxy.go,
// and other examples.
// Based on REDIRECT.
func SniffIptables(str *ugate.Stream) (string, error) {
	if _, ok := str.Out.(*net.TCPConn); !ok {
		return "", errors.New("invalid connection for iptbles")
	}
	ta, err := getOriginalDst(str.Out.(*net.TCPConn))
	if err != nil {
		return "", err
	}
	str.DestAddr = ta

	return ta.String(), nil
}

const (
	SO_ORIGINAL_DST      = 80
	IP6T_SO_ORIGINAL_DST = 80
)

func getsockopt(s int, level int, optname int, optval unsafe.Pointer, optlen *uint32) (err error) {
	_, _, e := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT, uintptr(s), uintptr(level), uintptr(optname),
		uintptr(optval), uintptr(unsafe.Pointer(optlen)), 0)
	if e != 0 {
		return e
	}
	return
}

// Should be used only for REDIRECT capture.
func getOriginalDst(clientConn *net.TCPConn) (rawaddr *net.TCPAddr, err error) {
	// test if the underlying fd is nil
	remoteAddr := clientConn.RemoteAddr()
	if remoteAddr == nil {
		err = errors.New("fd is nil")
		return
	}

	// net.TCPConn.File() will cause the receiver's (clientConn) socket to be placed in blocking mode.
	// The workaround is to take the File returned by .File(), do getsockopt() to get the original
	// destination, then create a new *net.TCPConn by calling net.Conn.FileConn().  The new TCPConn
	// will be in non-blocking mode.  What a pain.
	clientConnFile, err := clientConn.File()
	if err != nil {
		return
	}
	defer	clientConnFile.Close()

	fd :=  int(clientConnFile.Fd())
	if err = syscall.SetNonblock(fd, true); err != nil {
		return
	}

	// Get original destination
	// this is the only syscall in the Golang libs that I can find that returns 16 bytes
	// Example result: &{Multiaddr:[2 0 31 144 206 190 36 45 0 0 0 0 0 0 0 0] Interface:0}
	// port starts at the 3rd byte and is 2 bytes long (31 144 = port 8080)
	// IPv6 version, didn't find a way to detect network family
	//addr, err := syscall.GetsockoptIPv6Mreq(int(clientConnFile.Fd()), syscall.IPPROTO_IPV6, IP6T_SO_ORIGINAL_DST)
	// IPv4 address starts at the 5th byte, 4 bytes long (206 190 36 45)
	v6 := clientConn.LocalAddr().(*net.TCPAddr).IP.To4() == nil
	if v6 {
		var addr syscall.RawSockaddrInet6
		var len uint32
		len = uint32(unsafe.Sizeof(addr))
		err = getsockopt(fd, syscall.IPPROTO_IPV6, IP6T_SO_ORIGINAL_DST,
			unsafe.Pointer(&addr), &len)
		if err != nil {
			return
		}
		ip := make([]byte, 16)
		for i, b := range addr.Addr {
			ip[i] = b
		}
		pb := *(*[2]byte)(unsafe.Pointer(&addr.Port))
		return &net.TCPAddr{
			IP:   ip,
			Port: int(pb[0])*256 + int(pb[1]),
		}, nil
	} else {
		var addr syscall.RawSockaddrInet4
		var len uint32
		len = uint32(unsafe.Sizeof(addr))
		err = getsockopt(fd, syscall.IPPROTO_IP, SO_ORIGINAL_DST,
			unsafe.Pointer(&addr), &len)
		if err != nil {
			return nil, os.NewSyscallError("getsockopt", err)
		}
		ip := make([]byte, 4)
		for i, b := range addr.Addr {
			ip[i] = b
		}
		pb := *(*[2]byte)(unsafe.Pointer(&addr.Port))
		return &net.TCPAddr{
			IP:   ip,
			Port: int(pb[0])*256 + int(pb[1]),
		}, nil
	}
}

//func isLittleEndian() bool {
//	var i int32 = 0x01020304
//	u := unsafe.Pointer(&i)
//	pb := (*byte)(u)
//	b := *pb
//	return (b == 0x04)
//}

//var (
//	NativeOrder binary.ByteOrder
//)
//
//func init() {
//	if isLittleEndian() {
//		NativeOrder = binary.LittleEndian
//	} else {
//		NativeOrder = binary.BigEndian
//	}
//}
