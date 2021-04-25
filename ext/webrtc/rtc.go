package webrtc

import (
	"log"
	"net"
	"strconv"
	"time"

	"github.com/pion/turn/v2"
)
// Add:
// - TURN - seems a much simpler and broader mechanism to create remote listeners
//  TCP is problematic, but QUIC seems possible.

// STUN/TURN/ICE - dealing with NATs and multi-network, alternative to SNI sniffing

// WebRTC data channels

// Protocols:
// - RTP/3550 - not used, WebRTC requires SRTP/3711
// - SRTCP - control portocol
// - SRTP null or AES-counter mode.
// - HMAC-SHA1 (160 bit truncated to 80 or 32)
// - ZRTP/6189 and MIKEY for master key
//


// InitTURN creates a TURN server for the connected mesh nodes.
// By default, it will only forward to the mesh.
//
// TURN uses a username(max 513B) and 'message integrity code'
// Username is included in each message, followed by a HMAC-SHA1 using
// username:realm:pass
//
// If realm is set, server will send a nonce and realm, which will be included
// For short term, the key is the pass.
// This is the last field ( maybe followed by fingerprint)
//
// First request doesn't include user/nonce.
//
// Responses use the same password, and are verified ( but not encrypted - DTLS is used on top)
func InitTURN(publicIP string) error {
	tcpListener, err := net.Listen("tcp4", "0.0.0.0:"+strconv.Itoa(3478))
	if err != nil {
		return err
	}

	udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(3478))
	if err != nil {
		return err
	}

	ts, err := turn.NewServer(turn.ServerConfig{
		Realm: "h.webinf.info",

		ChannelBindTimeout: 10 * time.Minute,

		// Set AuthHandler callback
		// This is called everytime a user tries to authenticate with the TURN server
		// Return the key for that user, or false when no user is found
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) {
			if true {
				return []byte("key"), true
			}
			return nil, false
		},
		// PacketConnConfigs is a list of UDP Listeners and the configuration around them
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: udpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(publicIP), // Claim that we are listening on IP passed by user (This should be your Public IP)
					Address:      "0.0.0.0",              // But actually be listening on every interface

				},

			},
		},	// ListenerConfig is a list of Listeners and the configuration around them
		ListenerConfigs: []turn.ListenerConfig{
			{
				Listener: tcpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(publicIP),
					Address:      "0.0.0.0",
				},
			},
		},
	})
	if err != nil {
		return err
	}
	log.Println("TURN: ", ts)

	return nil
}
//
//// Blocking server.
//// Issue: NetConn for a server needs to 'latch' to a port, to emulate a connection. Yet we want to use
//// the same port on server side.
//func (r *RTC) InitSCTPS(port int) error {
//	addr := net.UDPAddr{
//		IP:   net.IPv4(0, 0, 0, 0),
//		Port: port,
//	}
//
//	conn, err := net.ListenUDP("udp", &addr)
//	if err != nil {
//		return err
//	}
//	defer conn.Close()
//
//	config := sctp.Config{
//		NetConn:       &disconnectedPacketConn{pConn: conn},
//		LoggerFactory: logging.NewDefaultLoggerFactory(),
//	}
//	a, err := sctp.Server(config)
//	if err != nil {
//		return err
//	}
//
//	s, err := a.AcceptStream()
//
//	s.SetReliabilityParams(true, sctp.ReliabilityTypeTimed, 10)
//
//	s.WriteSCTP([]byte("hi"), sctp.PayloadTypeWebRTCBinary)
//	s.Write([]byte{1})
//	return nil
//}
//
//func (r *RTC) InitSCTP(addr string) error {
//	conn, err := net.Dial("udp", "127.0.0.1:5678")
//	if err != nil {
//		log.Panic(err)
//	}
//	defer conn.Close()
//
//	config := sctp.Config{
//		NetConn:       conn,
//		LoggerFactory: logging.NewDefaultLoggerFactory(),
//	}
//	a, err := sctp.Client(config)
//	if err != nil {
//		return err
//	}
//
//	s, err := a.OpenStream(0, sctp.PayloadTypeWebRTCBinary)
//
//	s.SetReliabilityParams(true, sctp.ReliabilityTypeTimed, 10)
//
//	s.WriteSCTP([]byte("hi"), sctp.PayloadTypeWebRTCBinary)
//	s.Write([]byte{1})
//	return nil
//}
//
//
//
//
//type disconnectedPacketConn struct { // nolint: unused
//	mu    sync.RWMutex
//	rAddr net.Addr
//	pConn net.PacketConn
//}
//
//// Read
//func (c *disconnectedPacketConn) Read(p []byte) (int, error) {
//	i, rAddr, err := c.pConn.ReadFrom(p)
//	if err != nil {
//		return 0, err
//	}
//
//	c.mu.Lock()
//	c.rAddr = rAddr
//	c.mu.Unlock()
//
//	return i, err
//}
//
//// Write writes len(p) bytes from p to the DTLS connection
//func (c *disconnectedPacketConn) Write(p []byte) (n int, err error) {
//	return c.pConn.WriteTo(p, c.RemoteAddr())
//}
//
//// Close closes the conn and releases any Read calls
//func (c *disconnectedPacketConn) Close() error {
//	return c.pConn.Close()
//}
//
//// LocalAddr is a stub
//func (c *disconnectedPacketConn) LocalAddr() net.Addr {
//	if c.pConn != nil {
//		return c.pConn.LocalAddr()
//	}
//	return nil
//}
//
//// RemoteAddr is a stub
//func (c *disconnectedPacketConn) RemoteAddr() net.Addr {
//	c.mu.RLock()
//	defer c.mu.RUnlock()
//	return c.rAddr
//}
//
//// SetDeadline is a stub
//func (c *disconnectedPacketConn) SetDeadline(t time.Time) error {
//	return nil
//}
//
//// SetReadDeadline is a stub
//func (c *disconnectedPacketConn) SetReadDeadline(t time.Time) error {
//	return nil
//}
//
//// SetWriteDeadline is a stub
//func (c *disconnectedPacketConn) SetWriteDeadline(t time.Time) error {
//	return nil
//}
