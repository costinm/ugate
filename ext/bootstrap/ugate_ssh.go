// +build !MIN

package bootstrap

import (
	"github.com/costinm/ugate/ext/ssh"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		qa, _ := ssh.NewSSHTransport(ug, ug.Auth)

		return func(ug *ugatesvc.UGate) {
			qa.Start()
		}
	})
}
