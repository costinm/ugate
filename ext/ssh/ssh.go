package ssh

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/auth"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"golang.org/x/crypto/ssh"
)

type SSHTransport struct {
	Prefix string
	// Could be just an interface
	gate         *ugatesvc.UGate
	serverConfig *ssh.ServerConfig
	clientConfig *ssh.ClientConfig
	signer       ssh.Signer
}

type SSHConn struct {
	// ServerConn - also has Permission
	sc *ssh.ServerConn
	// SSH Client - only when acting as Dial
	// few internal fields in addition to ssh.Conn:
	// - forwards
	// - channelHandlers
	scl *ssh.Client

	closed chan struct{}

	// Original con, with remote/local addr
	wsCon     net.Conn

	inChans   <-chan ssh.NewChannel
	req       <-chan *ssh.Request

	LastSeen    time.Time
	ConnectTime time.Time

	// Includes the private key of this node
	t         *SSHTransport // transport.Transport

	remotePub ssh.PublicKey
}

// errClosed is returned when trying to accept a stream from a closed connection
//var errClosed = errors.New("conn closed")
const sshVersion = "SSH-2.0-dmesh"

// While the low-level H2 framing is in progress, use SSH as a multiplexer with push
// capabilities, using the built-in libraries.
// This may be removed once H2 framin wiht push is available.
// NewWsSshTransport creates a new transport using Websocket and SSH
// Based on QUIC transport.
//
func NewSSHTransport(ug *ugatesvc.UGate, auth *auth.Auth) (*SSHTransport, error) {
	signer, _ := ssh.NewSignerFromKey(auth.Cert.PrivateKey)

	return &SSHTransport{
		gate: ug,
		signer: signer,
		clientConfig: &ssh.ClientConfig{
			Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return nil
			},
			Config: ssh.Config{
				MACs: []string{
					"hmac-sha2-256-etm@openssh.com",
					"hmac-sha2-256",
					"hmac-sha1",
					"hmac-sha1-96",
				},
				Ciphers: []string{
					"aes128-gcm@openssh.com",
					"chacha20-poly1305@openssh.com",
					"aes128-ctr", "none",
				},
			},
		},
		serverConfig: &ssh.ServerConfig{
			PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
				return nil, fmt.Errorf("password rejected for %q", c.User())
			},
			ServerVersion: sshVersion,
			//PublicKeyCallback: sshGate.authPub,
			Config: ssh.Config{
				// none is not included
				MACs: []string{
					"hmac-sha2-256-etm@openssh.com",
					"hmac-sha2-256",
					"hmac-sha1",
					"hmac-sha1-96",
				},
				Ciphers: []string{
					"aes128-gcm@openssh.com",
					"chacha20-poly1305@openssh.com",
					"aes128-ctr", "none",
				},
			},
		},
	}, nil
}

func SignSSHHost(a *auth.Auth, hn string) {

}

func SignSSHUser(a *auth.Auth, hn string) {

}

// NewConn wraps a net.Conn using SSH for MUX and security.
func (t *SSHTransport) NewConn(nc net.Conn, isServer bool) (*SSHConn, error) {
	c := &SSHConn{
		closed: make(chan struct{}),
		t: t,
		wsCon:  nc,
	}
	c.ConnectTime = time.Now()

	if isServer {
		sc := &ssh.ServerConfig{
			Config:        t.serverConfig.Config,
			ServerVersion: sshVersion,
			PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
				c.remotePub = key
				return &ssh.Permissions{}, nil
			},
		}
		sc.AddHostKey(t.signer)
		conn, chans, globalSrvReqs, err := ssh.NewServerConn(nc, sc)
		if err != nil {
			return nil, err
		}
		c.sc =     conn
		c.inChans = chans
		c.req = globalSrvReqs
		// From handshake
	} else {
		cc, chans, reqs, err := ssh.NewClientConn(nc, "", &ssh.ClientConfig{
			Auth: t.clientConfig.Auth,
			Config: t.clientConfig.Config,
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				c.remotePub = key
				return nil
			},
		})
		if err != nil {
			return nil, err
		}
		client := ssh.NewClient(cc, chans, reqs)
		c.scl = client
		// The client adds "forwarded-tcpip" and "forwarded-streamlocal" when ListenTCP is called.
		// This in turns sends "tcpip-forward" command, with IP:port
		// The method returns a Listener, with port set.
		rawCh := client.HandleChannelOpen("raw")
		go func() {
			for inCh := range rawCh {
				log.Println("RAW CHAN", inCh)

			}
		}()
		c.inChans = chans
	}

	// At this point we have remotePub
	// It can be a *ssh.Certificate or ssh.CryptoPublicKey
	//

	go func() {
		for sshc := range c.inChans {
			switch sshc.ChannelType() {
			case "direct-tcpip":
				// Ignore 'ExtraData' containing Raddr, Rport, Laddr, Lport
				acc, r, _ := sshc.Accept()
				// Ignore in-band meta
				go ssh.DiscardRequests(r)

				s := ugate.NewStream()
				s.Out = acc
				s.In = acc
				t.gate.HandleVirtualIN(s)
			}
		}
	}()

	// Handle global requests - keepalive.
	// This does not support "-R" - use high level protocol
	go func() {
		for r := range c.req {
			// Global types.
			switch r.Type {
			case "keepalive@openssh.com":
				c.LastSeen = time.Now()
				//log.Println("SSHD: client keepalive", n.VIP)
				r.Reply(true, nil)

			default:
				log.Println("SSHD: unknown global REQUEST ", r.Type)
				if r.WantReply {
					log.Println(r.Type)
					r.Reply(false, nil)
				}

			}
		}
	}()

	return c, nil

}

func (t *SSHTransport) Start() {
	s := &Server{
		Port: t.gate.Config.BasePort + 22,
		Shell: "/bin/bash",
	}

	go s.ListenAndServe(t.signer)
}

// OpenStream creates a new stream.
// This uses the same channel in both directions.
func (c *SSHConn) OpenStream() (*sshstream, error) {
	if c.sc != nil {
		s, r, err := c.sc.OpenChannel("direct-tcpip", []byte{})
		if err != nil {
			return nil, err
		}
		go ssh.DiscardRequests(r)
		return &sshstream{Channel: s, con: c}, nil
	} else {
		s, r, err := c.scl.OpenChannel("direct-tcpip", []byte{})
		if err != nil {
			return nil, err
		}
		go ssh.DiscardRequests(r)
		return &sshstream{Channel: s, con: c}, nil
	}
}

// Also implements ssh.Channel - add SendRequest and Stderr, as well as CloseWrite
type sshstream struct {
	ssh.Channel
	con *SSHConn
}





