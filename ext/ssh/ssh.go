package ssh

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/costinm/ugate/pkg/auth"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type Server struct {
	Port           int
	Shell          string
	AuthorizedKeys []ssh.PublicKey

	// Could be just an interface
	gate         *ugatesvc.UGate

	clientConfig *gossh.ClientConfig

	signer       ssh.Signer

	// HandleConn can be used to overlay a SSH conn.
	server       *ssh.Server
}

func NewSSHTransport(ug *ugatesvc.UGate, auth *auth.Auth) (*Server, error) {
	signer, _ := gossh.NewSignerFromKey(auth.Cert.PrivateKey)

	s := &Server{
		gate: ug,
		signer: signer,
		clientConfig: &gossh.ClientConfig{
			Auth: []gossh.AuthMethod{gossh.PublicKeys(signer)},
			HostKeyCallback: func(hostname string, remote net.Addr, key gossh.PublicKey) error {
				return nil
			},
			//Config: gossh.Config{
			//	MACs: []string{
			//		"hmac-sha2-256-etm@opengossh.com",
			//		"hmac-sha2-256",
			//		"hmac-sha1",
			//		"hmac-sha1-96",
			//	},
			//	Ciphers: []string{
			//		"aes128-gcm@opengossh.com",
			//		"chacha20-poly1305@opengossh.com",
			//		"aes128-ctr", "none",
			//	},
			//},
		},
		Port: ug.Config.BasePort + 22,
		Shell: "/bin/bash",
	}
	pk, err := LoadAuthorizedKeys(os.Getenv("HOME") + "/.ssh/authorized_keys")
	if err == nil {
		s.AuthorizedKeys = pk
	}
	root := os.Getenv("ROOT")
	if root != "" {
		// Expect ECDSA as base64
		k1 := "ecdsa-sha2-nistp256 " + SSH_ECPREFIX + root + " " + auth.Name + "@" + auth.Domain
		//publicUncomp, _ := base64.RawURLEncoding.DecodeString(root)
		//x, y := elliptic.Unmarshal(elliptic.P256(), publicUncomp)
		//pubkey := ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

		pubk, _,_, _, err := ssh.ParseAuthorizedKey([]byte(k1))
		if err == nil {
			s.AuthorizedKeys = append(s.AuthorizedKeys, pubk)
		}

		k1 = "cert-authority " + k1

		pubk, _,_, _, err = ssh.ParseAuthorizedKey([]byte(k1))
		if err == nil {
			s.AuthorizedKeys = append(s.AuthorizedKeys, pubk)
		}
	}

	s.server = s.getServer(signer)

	return s, nil
}

func (srv *Server) getServer(signer ssh.Signer) *ssh.Server {
	forwardHandler := &ssh.ForwardedTCPHandler{}

	server := &ssh.Server{
		Addr:    fmt.Sprintf(":%d", srv.Port),
		Handler: srv.connectionHandler,
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"direct-tcpip": ssh.DirectTCPIPHandler,
			"session":      ssh.DefaultSessionHandler,
		},
		LocalPortForwardingCallback: ssh.LocalPortForwardingCallback(func(ctx ssh.Context, dhost string, dport uint32) bool {
			log.Println("Accepted forward", dhost, dport)
			return true
		}),
		ReversePortForwardingCallback: ssh.ReversePortForwardingCallback(func(ctx ssh.Context, host string, port uint32) bool {
			log.Println("attempt to bind", host, port, "granted")
			return true
		}),
		RequestHandlers: map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		},
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": sftpHandler,
		},
	}

	//server.SetOption(ssh.HostKeyPEM([]byte(hostKeyBytes)))
	server.AddHostKey(signer)

	if srv.AuthorizedKeys != nil {
		server.PublicKeyHandler = srv.authorize
	}

	return server
}

func (t *Server) Start() {
	go t.server.ListenAndServe()
}







