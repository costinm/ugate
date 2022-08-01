package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"log"
	"math/big"
	"testing"
	"time"
)

// TODO: move the old tests for auth here.

const (
	testpriv = "bSaKOws92sj2DdULvWSRN3O03a5vIkYW72dDJ_TIFyo"
	testpub  = "BALVohWt4pyr2L9iAKpJig2mJ1RAC1qs5CGLx4Qydq0rfwNblZ5IJ5hAC6-JiCZtwZHhBlQyNrvmV065lSxaCOc"
)

func TestVapid(t *testing.T) {
	rfcEx := "vapid t=eyJ0eXAiOiJKV1QiLCJhbGciOiJFUzI1NiJ9.eyJhdWQiOiJodHRwczovL3B1c2guZXhhbXBsZS5uZXQiLCJleHAiOjE0NTM1MjM3NjgsInN1YiI6Im1haWx0bzpwdXNoQGV4YW1wbGUuY29tIn0.i3CYb7t4xfxCDquptFOepC9GAu_HLGkMlMuCGSK2rpiUfnK9ojFwDXb1JrErtmysazNjjvW2L9OkSSHzvoD1oA, " +
		"k=BA1Hxzyi1RUM1b5wjxsn7nGxAszw2u61m164i3MrAIxHF6YK5h4SDYic-dRuU_RCPCfA5aq9ojSwk5Y2EmClBPs"

	rfcT, rfcP, err := CheckVAPID(rfcEx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range rfcT.Aud {
		if a != "https://push.example.net" {
			t.Fatal("Aud got ", rfcT.Aud)
		}
	}
	log.Println(len(rfcP), rfcT)

	alice := NewAuth(nil, "", "test.sender")
	bobToken := alice.VAPIDToken("bob")
	log.Println("Authorization: " + bobToken)

	tok, pub, err := CheckVAPID(bobToken, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	log.Println(len(pub), tok)

	btb := []byte(bobToken)
	btb[50]++
	bobToken = string(btb)
	_, _, err = CheckVAPID(bobToken, time.Now())
	if err == nil {
		t.Fatal("Expecting error")
	}

}

func TestSigFail(t *testing.T) {
	payload := `{"UA":"22-palman-LG-V510-","IP4":"10.1.10.223"}`
	log.Println(payload)

	payloadhex, _ := hex.DecodeString("7b225541223a2232322d70616c6d616e2d4c472d563531302d222c22495034223a2231302e312e31302e323233227d0a9d4eda35ad1bba104bfee8f92c3d602ceb6f53754a499e28d5569c5a7173b2c100f9a1d4d19f1154cf2699df676fcd63ddd3bf6cd5e1a4db9bccceec262c0be1")
	log.Println(string(payloadhex[0 : len(payloadhex)-64]))

	//BJ1O2jWtG7oQS/7o+Sw9YCzrb1N1SkmeKNVWnFpxc7LBAPmh1NGfEVTPJpnfZ2/NY93Tv2zV4aTbm8zO7CYsC+E=
	log.Println("Pub:", hex.EncodeToString(payloadhex[len(payloadhex)-64:]))
	log.Println("Pub:", "9d4eda35ad1bba104bfee8f92c3d602ceb6f53754a499e28d5569c5a7173b2c100f9a1d4d19f1154cf2699df676fcd63ddd3bf6cd5e1a4db9bccceec262c0be1")
	//buf := bytes.RBuffer{}
	//buf.Write(payloadhex)
	//buf.Write(pub)

	hasher := crypto.SHA256.New()
	hasher.Write(payloadhex) //[0:64]) // only public key, for debug
	hash := hasher.Sum(nil)
	log.Println("SHA:", hex.EncodeToString(hash))

	sha := "a2fe666ae95fe8b7c05bfb0215c9d58fe2121ec0baef70de8cc5fd10d15a3e9c"
	log.Println("SHA:", sha)

	sig, _ := hex.DecodeString("9930116d656c7b977a46ca948eb7c49f0fe9b4fe11ae3790bbd8ed47d71135278ddda2d3f9b1aafdad08a14e38b5fc71e41527b0aecda7ce307ef23a8f0f8ee1")

	ok := Verify(payloadhex, payloadhex[len(payloadhex)-64:], sig)
	log.Println(ok)

}

func TestSig(t *testing.T) {
	pubb, _ := base64.RawURLEncoding.DecodeString(testpub)
	priv, _ := base64.RawURLEncoding.DecodeString(testpriv)
	d := new(big.Int).SetBytes(priv)

	log.Println("Pub: ", hex.EncodeToString(pubb))
	x, y := elliptic.Unmarshal(Curve256, pubb)
	pubkey := ecdsa.PublicKey{Curve: Curve256, X: x, Y: y}

	pkey := ecdsa.PrivateKey{PublicKey: pubkey, D: d}

	hasher := crypto.SHA256.New()
	hasher.Write(pubb[1:65])
	hash := hasher.Sum(nil)
	log.Println("HASH: ", hex.EncodeToString(hash))

	r, s, _ := ecdsa.Sign(rand.Reader, &pkey, hash)
	rBytes := r.Bytes()
	rBytesPadded := make([]byte, 32)
	copy(rBytesPadded[32-len(rBytes):], rBytes)

	sBytes := s.Bytes()
	sBytesPadded := make([]byte, 32)
	copy(sBytesPadded[32-len(sBytes):], sBytes)
	sig := append(rBytesPadded, sBytesPadded...)

	log.Println(pubkey)

	log.Println("R:", hex.EncodeToString(r.Bytes()), hex.EncodeToString(s.Bytes()))

	err := Verify(pubb[1:65], pubb[1:65], sig)
	if err != nil {
		t.Error(err)
	}
}

var Curve256 = elliptic.P256()

// ~31us on amd64/2G
func BenchmarkSig(b *testing.B) {
	pubb, _ := base64.RawURLEncoding.DecodeString(testpub)
	priv, _ := base64.RawURLEncoding.DecodeString(testpriv)
	d := new(big.Int).SetBytes(priv)
	x, y := elliptic.Unmarshal(Curve256, pubb)
	pubkey := ecdsa.PublicKey{Curve: Curve256, X: x, Y: y}
	pkey := ecdsa.PrivateKey{PublicKey: pubkey, D: d}

	for i := 0; i < b.N; i++ {
		hasher := crypto.SHA256.New()
		hasher.Write(pubb[1:65])
		ecdsa.Sign(rand.Reader, &pkey, hasher.Sum(nil))
	}
}

// 2us
func BenchmarkVerify(b *testing.B) {
	pubb, _ := base64.RawURLEncoding.DecodeString(testpub)
	priv, _ := base64.RawURLEncoding.DecodeString(testpriv)
	d := new(big.Int).SetBytes(priv)
	x, y := elliptic.Unmarshal(Curve256, pubb)
	pubkey := ecdsa.PublicKey{Curve: Curve256, X: x, Y: y}
	pkey := ecdsa.PrivateKey{PublicKey: pubkey, D: d}
	hasher := crypto.SHA256.New()
	hasher.Write(pubb[1:65])
	r, s, _ := ecdsa.Sign(rand.Reader, &pkey, hasher.Sum(nil))
	rBytes := r.Bytes()
	rBytesPadded := make([]byte, 32)
	copy(rBytesPadded[32-len(rBytes):], rBytes)

	sBytes := s.Bytes()
	sBytesPadded := make([]byte, 32)
	copy(sBytesPadded[32-len(sBytes):], sBytes)
	sig := append(rBytesPadded, sBytesPadded...)

	for i := 0; i < b.N; i++ {
		Verify(pubb, pubb, sig)
	}
}
