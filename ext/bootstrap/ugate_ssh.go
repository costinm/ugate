package bootstrap

import (
	"context"
	"crypto/ecdsa"
	"io"
	"net"

	"github.com/costinm/ssh-mesh/ssh"
	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/ugatesvc"
	gossh "golang.org/x/crypto/ssh"
)

func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		ca := &ssh.SSHCA{}
		// TODO: if CA_ADDR set get certs from remote
		// if root CA present - act as a CA
		ca.LoadRoot(ug.Auth.Cert.PrivateKey.(*ecdsa.PrivateKey))

		signer, _ := gossh.NewSignerFromKey(ug.Auth.Cert.PrivateKey)

		qa, _ := ssh.NewSSHTransport(&ssh.TransportConfig{
			SignerHost:   signer,
			SignerClient: signer,
		})

		// Forward requests use ugate Dialer.
		qa.Forward = func(ctx context.Context, host string, closer io.ReadWriteCloser) {
			str := ugate.GetStream(closer, closer)
			defer ug.OnStreamDone(str)
			ug.OnStream(str)

			str.Dest = host
			str.Direction = ugate.StreamTypeOut
			str.PostDialHandler = func(conn net.Conn, err error) {
				if err != nil {
					//w.WriteHeader(503)
					//w.Write([]byte("Dial error" + err.Error()))
					return
				}
				//proxyClient.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))
			}
			// TODO: add sniffing on the outbound

			ug.DialAndProxy(str)
		}

		// TODO: hook hbone - forward connections to other side.

		return func(ug *ugatesvc.UGate) {
			qa.Start()
		}
	})
}
