package ugate

import (
	"crypto"
	"net"
	"testing"
)

func testKey(k crypto.PrivateKey, t *testing.T) {
	pk := PublicKey(k)
	if pk == nil {
		t.Fatal("Invalid public")
	}
	id := IDFromPublicKey(pk)

	kb, err := MarshalPrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	_, err = UnmarshalPrivateKey(kb)
	if err != nil {
		t.Fatal(err)
	}


	crt, err := KeyToCertificate(k)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := RawToCertChain(crt.Certificate)
	if err != nil {
		t.Fatal(err)
	}
	certPub, err := PubKeyFromCertChain(chain)
	if id != IDFromPublicKey(certPub) {
		t.Fatal("Cert chain public key not matching", id, IDFromPublicKey(certPub))
	}

	t.Log("Key ID:", id)

	ug := NewGate(&net.Dialer{})
	_, _, _ = ug.Add(&ListenerConf{

	})

}

func TestCrypto(t *testing.T) {
	t.Run("ED25519", func(t *testing.T) {
		ed := GenerateKeyPair()
		testKey(ed, t)

		al := GenerateKeyPair()
		testKey(al, t)


	})
	t.Run("RSA", func(t *testing.T) {
		ed := GenerateRSAKeyPair()
		testKey(ed, t)
	})
	t.Run("EC256", func(t *testing.T) {
		ed := GenerateEC256KeyPair()
		testKey(ed, t)
	})

}


func TestUGate(t *testing.T) {
	td := &net.Dialer{}
	ug := NewGate(td)

	ug.Add(&ListenerConf{
		Port: 3000,
		Protocol: "echo",
	})
	ug.Add(&ListenerConf{
		Port: 3001,
		Protocol: "static",
	})
	ug.Add(&ListenerConf{
		Port: 3002,
		Protocol: "delay",
	})
	ug.Add(&ListenerConf{
		Port: 3006,
		Protocol: "sni",
	})
	ug.Add(&ListenerConf{
		Port: 3003,
		Local: "127.0.0.1:3003",
		Protocol: "socks5",
	})
	// In-process dialer (ssh, etc)
	ug.Add(&ListenerConf{
		Port:   3004,
		Dialer: td,
	})
	ug.Add(&ListenerConf{
		Port: 3005,
		Remote: "localhost:3000",
	})

	ug.Add(&ListenerConf{
		Port: 3006,
		Handler: &EchoHandler{},
	})

}

func BenchmarkUGate(t *testing.B) {

}
