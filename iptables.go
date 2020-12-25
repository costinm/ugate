package ugate

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"syscall"
)

//

// Status:
// - TCP capture with redirect works
// - capture with TPROXY is not possible - TPROXY is on the PREROUTING chain only,
// not touched by output packets.
//   https://upload.wikimedia.org/wikipedia/commons/3/37/Netfilter-packet-flow.svg
//
// It works great for transparent proxy in a gateway/router - however same can be also done using the TUN
// and routing to the TUN using iptables or other means.


// ServeConn is used to serve a single TCP UdpNat.
// See https://github.com/cybozu-go/transocks
// https://github.com/ryanchapman/go-any-proxy/blob/master/any_proxy.go,
// and other examples.
// Based on REDIRECT.
func (ug *UGate) sniffIptables(conn *RawConn, proto string) error {
	addr, port, conn1, err := getOriginalDst(conn.raw.(*net.TCPConn))
	if err != nil {
		conn.Close()
		return err
	}

	iaddr := net.IP(addr)

	ta := net.TCPAddr{IP: iaddr, Port: int(port)}
	conn.Meta().Target = ta.String()
	conn.Stats.Type = proto
	// Needs to be replaced, original has been changed
	conn.raw = conn1
	//log.Println("IPT ", proto, ta, conn.RemoteAddr())

	return nil
}

const (
	SO_ORIGINAL_DST      = 80
	IP6T_SO_ORIGINAL_DST = 80
)

// Should be used only for REDIRECT capture.
func getOriginalDst(clientConn *net.TCPConn) (rawaddr []byte, port uint16, newTCPConn *net.TCPConn, err error) {

	if clientConn == nil {
		err = errors.New("ERR: clientConn is nil")
		return
	}

	// test if the underlying fd is nil
	remoteAddr := clientConn.RemoteAddr()
	if remoteAddr == nil {
		err = errors.New("ERR: clientConn.fd is nil")
		return
	}

	//srcipport := fmt.Sprintf("%v", clientConn.RemoteAddr())

	newTCPConn = nil
	// net.TCPConn.File() will cause the receiver's (clientConn) socket to be placed in blocking mode.
	// The workaround is to take the File returned by .File(), do getsockopt() to get the original
	// destination, then create a new *net.TCPConn by calling net.Conn.FileConn().  The new TCPConn
	// will be in non-blocking mode.  What a pain.
	clientConnFile, err := clientConn.File()
	if err != nil {
		//common.Errorf("GETORIGINALDST|%v->?->FAILEDTOBEDETERMINED|ERR: could not get a copy of the client UdpNat's file object", srcipport)
		return
	} else {
		clientConn.Close()
	}

	// Get original destination
	// this is the only syscall in the Golang libs that I can find that returns 16 bytes
	// Example result: &{Multiaddr:[2 0 31 144 206 190 36 45 0 0 0 0 0 0 0 0] Interface:0}
	// port starts at the 3rd byte and is 2 bytes long (31 144 = port 8080)
	// IPv6 version, didn't find a way to detect network family
	//addr, err := syscall.GetsockoptIPv6Mreq(int(clientConnFile.Fd()), syscall.IPPROTO_IPV6, IP6T_SO_ORIGINAL_DST)
	// IPv4 address starts at the 5th byte, 4 bytes long (206 190 36 45)
	addr, err := syscall.GetsockoptIPv6Mreq(int(clientConnFile.Fd()), syscall.IPPROTO_IP, SO_ORIGINAL_DST)
	if err != nil {
		return
	}
	newConn, err := net.FileConn(clientConnFile)
	if err != nil {
		return
	}
	if _, ok := newConn.(*net.TCPConn); ok {
		newTCPConn = newConn.(*net.TCPConn)
		clientConnFile.Close()
	} else {
		errmsg := fmt.Sprintf("ERR: newConn is not a *net.TCPConn, instead it is: %T (%v)", newConn, newConn)
		err = errors.New(errmsg)
		return
	}

	// \attention: IPv4 only!!!
	// address type, 1 - IPv4, 4 - IPv6, 3 - hostname, only IPv4 is supported now
	rawaddr = make([]byte, 4)
	// raw IP address, 4 bytes for IPv4 or 16 bytes for IPv6, only IPv4 is supported now
	copy(rawaddr, addr.Multiaddr[4:8])

	// Bigendian is the network bit order, seems to be used here.
	port = binary.BigEndian.Uint16(addr.Multiaddr[2:])

	return
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
