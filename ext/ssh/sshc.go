package ssh

import (
	"log"
	"net"
	"time"

	"github.com/costinm/ugate"
	gossh "golang.org/x/crypto/ssh"
)

type SSHConn struct {
	// ServerConn - also has Permission
	sc *gossh.ServerConn
	// SSH Client - only when acting as Dial
	// few internal fields in addition to ssh.Conn:
	// - forwards
	// - channelHandlers
	scl *gossh.Client

	closed chan struct{}

	// Original con, with remote/local addr
	wsCon     net.Conn

	inChans   <-chan gossh.NewChannel
	req       <-chan *gossh.Request

	LastSeen    time.Time
	ConnectTime time.Time

	// Includes the private key of this node
	t         *Server // transport.Transport

	remotePub gossh.PublicKey
}


// NewConn wraps a net.Conn using SSH for MUX and security.
func (t *Server) NewConn(nc net.Conn, isServer bool) (*SSHConn, error) {
	c := &SSHConn{
		closed: make(chan struct{}),
		t: t,
		wsCon:  nc,
	}
	c.ConnectTime = time.Now()

	if isServer {
		// server.ConnCallback will be called with a context
		// the ssh con will be stored in the context.
		go t.server.HandleConn(nc)
	} else {
		cc, chans, reqs, err := gossh.NewClientConn(nc, "", &gossh.ClientConfig{
			Auth: t.clientConfig.Auth,
			Config: t.clientConfig.Config,
			HostKeyCallback: func(hostname string, remote net.Addr, key gossh.PublicKey) error {
				c.remotePub = key
				return nil
			},
		})
		if err != nil {
			return nil, err
		}
		client := gossh.NewClient(cc, chans, reqs)
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
	// It can be a *gossh.Certificate or gossh.CryptoPublicKey
	//

	go func() {
		for sshc := range c.inChans {
			switch sshc.ChannelType() {
			case "direct-tcpip":
				// Ignore 'ExtraData' containing Raddr, Rport, Laddr, Lport
				acc, r, _ := sshc.Accept()
				// Ignore in-band meta
				go gossh.DiscardRequests(r)

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
// OpenStream creates a new stream.
// This uses the same channel in both directions.
func (c *SSHConn) OpenStream() (*sshstream, error) {
	if c.sc != nil {
		s, r, err := c.sc.OpenChannel("direct-tcpip", []byte{})
		if err != nil {
			return nil, err
		}
		go gossh.DiscardRequests(r)
		return &sshstream{Channel: s, con: c}, nil
	} else {
		s, r, err := c.scl.OpenChannel("direct-tcpip", []byte{})
		if err != nil {
			return nil, err
		}
		go gossh.DiscardRequests(r)
		return &sshstream{Channel: s, con: c}, nil
	}
}

// Also implements gossh.Channel - add SendRequest and Stderr, as well as CloseWrite
type sshstream struct {
	gossh.Channel
	con *SSHConn
}


