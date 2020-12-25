package ugate

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io/ioutil"

	"errors"
	"fmt"
	"math/big"
	"time"
)

// Forked from go-libp2p-tls:
// - use normal SANs
// - use normal ALPN (H2)
// - not used with upgrader or ipfs negotiation - using standard protocols
//

// Private key, certificates and associated helpers.
// Peer ID is 32 bytes - ED25519 public key or sha of RSA or EC256.
// An IPv6 address is also derived from the public key.
type Auth struct {

	// Primary public key of the node.
	// EC256: 65 bytes, uncompressed format
	// RSA: DER
	// ED25519: 32B
	Pub []byte

	// Private key to use in both server and client authentication. This is the base of the VIP of the node.
	// ED22519: 32B
	// EC256: DER
	// RSA: DER
	Priv []byte

	PrivateKey crypto.PrivateKey
	PublicKey crypto.PublicKey

	Cert *x509.Certificate
}

var certValidityPeriod = 100 * 365 * 24 * time.Hour // ~100 years


func RawToCertChain(rawCerts [][]byte) ([]*x509.Certificate, error){
	chain := make([]*x509.Certificate, len(rawCerts))
	for i := 0; i < len(rawCerts); i++ {
		cert, err := x509.ParseCertificate(rawCerts[i])
		if err != nil {
			return nil, err
		}
		chain[i] = cert
	}
	return chain, nil
}


// PubKeyFromCertChain verifies the certificate chain and extract the remote's public key.
func PubKeyFromCertChain(chain []*x509.Certificate) (crypto.PublicKey, error) {
	cert := chain[0]

	// Self-signed certificate
	if len(chain) == 1 {
		pool := x509.NewCertPool()
		pool.AddCert(cert)
		if _, err := cert.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
			// If we return an x509 error here, it will be sent on the wire.
			// Wrap the error to avoid that.
			return nil, fmt.Errorf("certificate verification failed: %s", err)
		}
	} else {
		//
		pool := x509.NewCertPool()
		pool.AddCert(chain[len(chain)-1])
		if _, err := cert.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
			// If we return an x509 error here, it will be sent on the wire.
			// Wrap the error to avoid that.
			return nil, fmt.Errorf("chain certificate verification failed: %s", err)
		}
	}

	// IPFS uses a key embedded in a custom extension, and verifies the public key of the cert is signed
	// with the node public key

	// This transport is instead based on standard certs/TLS

	key := cert.PublicKey
	if ec, ok := key.(*ecdsa.PublicKey); ok {
		return ec, nil
	}
	if rsak, ok := key.(*rsa.PublicKey); ok {
		return rsak, nil
	}
	if ed, ok := key.(ed25519.PublicKey); ok {
		return ed, nil
	}

	return nil, errors.New("Unknown public key")
}

func KeyToCertificate(sk crypto.PrivateKey) (*tls.Certificate, error) {

	//certKeyPub, err := x509.MarshalPKIXPublicKey(certKey.Public())
	//if err != nil {
	//	return nil, err
	//}
	//signature, err := sk.Sign(append([]byte(certificatePrefix), certKeyPub...))
	//if err != nil {
	//	return nil, err
	//}
	//value, err := asn1.Marshal(signedKey{
	//	PubKey:    keyBytes,
	//	Signature: signature,
	//})
	//if err != nil {
	//	return nil, err
	//}

	sn, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: sn,
		NotBefore:    time.Time{},
		NotAfter:     time.Now().Add(certValidityPeriod),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl,
		PublicKey(sk), sk)
	if err != nil {
		return nil, err
	}

	return &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  sk,
	}, nil
}

//// We want nodes without AES hardware (e.g. ARM) support to always use ChaCha.
//// Only if both nodes have AES hardware support (e.g. x86), AES should be used.
//// x86->x86: AES, ARM->x86: ChaCha, x86->ARM: ChaCha and ARM->ARM: Chacha
//// This function returns true if we don't have AES hardware support, and false otherwise.
//// Thus, ARM servers will always use their own cipher suite preferences (ChaCha first),
//// and x86 servers will aways use the client's cipher suite preferences.
//func preferServerCipherSuites() bool {
//	// Copied from the Go TLS implementation.
//
//	// Check the cpu flags for each platform that has optimized GCM implementations.
//	// Worst case, these variables will just all be false.
//	var (
//		hasGCMAsmAMD64 = cpu.X86.HasAES && cpu.X86.HasPCLMULQDQ
//		hasGCMAsmARM64 = cpu.ARM64.HasAES && cpu.ARM64.HasPMULL
//		// Keep in sync with crypto/aes/cipher_s390x.go.
//		hasGCMAsmS390X = cpu.S390X.HasAES && cpu.S390X.HasAESCBC && cpu.S390X.HasAESCTR && (cpu.S390X.HasGHASH || cpu.S390X.HasAESGCM)
//
//		hasGCMAsm = hasGCMAsmAMD64 || hasGCMAsmARM64 || hasGCMAsmS390X
//	)
//	return !hasGCMAsm
//}

func IDFromPublicKey(key crypto.PublicKey) string {
	m := MarshalPublicKey(key)
	return base64.RawURLEncoding.EncodeToString(m)
}

func GetOrGenerateCert(fname string, priv crypto.PrivateKey) ([]byte, error) {
	if fname != "" {
		ex, err := ioutil.ReadFile(fname)
		if err == nil {
			certPEMBlock, _ := pem.Decode(ex)
			return certPEMBlock.Bytes, nil
			//tlsCert, err := x509.ParseCertificates(ex)
			//if err != nil {
			//	return nil, err
			//}
			//
			//return tlsCert, nil
		}
	}
	crt, err := KeyToCertificate(priv)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: crt.Certificate[0]})

	if fname != "" {
		err = ioutil.WriteFile(fname, certPEM, 0700)
	}
	return crt.Certificate[0], err
}

func GetOrGenerateKey(fname string) (crypto.PrivateKey, error) {
	if fname != "" {
		ex, err := ioutil.ReadFile(fname)
		if err == nil {
			key, err := UnmarshalPrivateKey(ex)
			if err != nil {
				return nil, err
			}
			return key, nil
		}
	}
	key := GenerateKeyPair()
	data, _ := MarshalPrivateKey(key)
	var err error
	if fname != "" {
		err = ioutil.WriteFile(fname, data, 0700)
	}
	return key, err
}

func GenerateKeyPair() ed25519.PrivateKey {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	return priv
}

func GenerateRSAKeyPair() *rsa.PrivateKey {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	return priv
}

func GenerateEC256KeyPair() *ecdsa.PrivateKey {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return priv
}

// Serialize a privte key. Will use the SSH format, for more interop.
// A lot of apps use SSH public keys for access.
// Supports RSA, EC256 and ED25519
func MarshalPrivateKey(key crypto.PrivateKey) ([]byte, error) {
	if k, ok := key.(ed25519.PrivateKey); ok {
		return []byte(k), nil
	}
	if k, ok := key.(*ecdsa.PrivateKey); ok {
		return x509.MarshalECPrivateKey(k)
	}
	if k, ok := key.(*rsa.PrivateKey); ok {
		bk := x509.MarshalPKCS1PrivateKey(k)
		return bk, nil
	}

	return nil, errors.New("unknown key")
}

func MarshalPublicKey(key crypto.PublicKey) []byte {
	if k, ok := key.(ed25519.PublicKey); ok {
		return []byte(k)
	}
	if k, ok := key.(*ecdsa.PublicKey); ok {
		return elliptic.Marshal(elliptic.P256(), k.X, k.Y)
		// starts with 0x04 == uncompressed curve
	}
	if k, ok := key.(*rsa.PublicKey); ok {
		bk := x509.MarshalPKCS1PublicKey(k)
		return bk
	}

	return nil
}


func PublicKey(key crypto.PrivateKey) crypto.PublicKey {
	if k, ok := key.(ed25519.PrivateKey); ok {
		return k.Public()
	}
	if k, ok := key.(*ecdsa.PrivateKey); ok {
		return k.Public()
	}
	if k, ok := key.(*rsa.PrivateKey); ok {
		return k.Public()
	}

	return nil
}

// Extract the key from the bytes.
// IPFS uses a protobuf, with keytype plus 'raw' encoding.
// The raw encoding is the 'natural' format.
func UnmarshalPrivateKey(rawKey []byte) (key crypto.PrivateKey, err error) {
	if len(rawKey) == 64 {
		key = ed25519.PrivateKey(rawKey)
		// public is the last 32 bytes
		return
	}

	if len(rawKey) < 256 {
		// der size: ~120 bytes ( including version(1), PrivateKey(32) curve oid(7), public key 65 bytes )
		key, err = x509.ParseECPrivateKey(rawKey)
		return key, err
	}

	// der size: ~1200 bytes
	key, err = x509.ParsePKCS1PrivateKey(rawKey)
	return
}
