package sshd

import (
	"io/ioutil"
	"os"
	"strconv"
)

// Helpers around sshd, using exec.

var SshdConfig = `
Port 15022
AddressFamily any
ListenAddress 0.0.0.0
ListenAddress ::
Protocol 2
LogLevel INFO

HostKey /tmp/sshd/ssh_host_ecdsa_key

PermitRootLogin yes

AuthorizedKeysFile	/tmp/sshd/authorized_keys

PasswordAuthentication yes
PermitUserEnvironment yes

AcceptEnv LANG LC_*
PrintMotd no
#UsePAM no

Subsystem	sftp	/usr/lib/openssh/sftp-server
`

type SSHDConfig struct {
	Port int
}

// StartSSHD will start sshd.
// If running as root, listens on port 22
// If started as user, listen on port 15022
func StartSSHD(cfg *SSHDConfig) {

	// /usr/sbin/sshd -p 15022 -e -D -h ~/.ssh/ec-key.pem
	// -f config
	// -c host_cert_file
	// -d debug - only one connection processed
	// -e debug to stderr
	// -h or -o HostKey
	// -p or -o Port
	//
	if cfg == nil {
		cfg = &SSHDConfig{}
	}
	if cfg.Port == 0 {
		cfg.Port = 15022
	}

	os.Mkdir("/tmp/sshd", 0700)

	os.StartProcess("/usr/bin/ssh-keygen",
		[]string{
			"-q",
			"-f",
			"/tmp/sshd/ssh_host_ecdsa_key",
			"-N",
			"",
			"-t",
			"ecdsa",
			},
		&os.ProcAttr{
		})

	ioutil.WriteFile("/tmp/sshd/sshd_confing", []byte(SshdConfig), 0700)

	os.StartProcess("/usr/sbin/sshd",
		[]string{"-f", "/tmp/sshd/sshd_config",
			"-e",
			"-D",
			"-p", strconv.Itoa(cfg.Port),
		}, nil)

}
