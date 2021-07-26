package ssh

import (
	"crypto"
	"crypto/dsa"
	"crypto/ecdsa"
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/costinm/ugate/pkg/auth"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"golang.org/x/crypto/ed25519"
)

const SSH_ECPREFIX = "AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABB"

// Convert from SSH to crypto
func SSHKey2Crypto(auth *auth.Auth, keyRSA []byte) (crypto.PrivateKey, error) {
	keyssh, err := gossh.ParseRawPrivateKey(keyRSA)
	switch key := keyssh.(type) {
	case *rsa.PrivateKey:
		// PRIVATE_KEY - may return RSA or ecdsa
		// RSA PRIVATE KEY
		//auth.RSAPrivate = key
		return key, nil
	case *ecdsa.PrivateKey:
		// EC PRIVATE KEY
		return key, nil
	case *dsa.PrivateKey:
		// DSA PRIVATE KEY
		return key, nil
	case *ed25519.PrivateKey:
		// OPENSSH PRIVATE KEY - may return rsa or ED25519
		//auth.EDPrivate = key
		return key, nil
	}

	return nil, err
}


func SignSSHHost(a *auth.Auth, hn string) {

}

func SignSSHUser(a *auth.Auth, hn string) {

}


// LoadAuthorizedKeys loads path as an array.
// It will return nil if path doesn't exist.
func LoadAuthorizedKeys(path string) ([]ssh.PublicKey, error) {
	authorizedKeysBytes, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	authorizedKeys := []ssh.PublicKey{}
	for len(authorizedKeysBytes) > 0 {
		pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			return nil, err
		}

		authorizedKeys = append(authorizedKeys, pubKey)
		authorizedKeysBytes = rest
	}

	if len(authorizedKeys) == 0 {
		return nil, fmt.Errorf("%s was empty", path)
	}

	return authorizedKeys, nil
}

