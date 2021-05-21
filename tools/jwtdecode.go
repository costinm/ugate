package main

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/costinm/ugate/pkg/auth"
)

var (
	jwt  = flag.String("jwt", "", "JWT to decode")
	aud  = flag.String("aud", "", "Aud to check")

)
func main() {
	flag.Parse()
	decode(*jwt, *aud)
}

// Decode a JWT.
// If crt is specified - verify it using that cert
func decode(jwt, aud string) {
	// TODO: verify if it's a VAPID
	parts := strings.Split(jwt, ".")
	p1b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println(string(p1b))

	scrt, _ := ioutil.ReadFile("server.crt")
	block, _ := pem.Decode(scrt)
	xc, _ := x509.ParseCertificate(block.Bytes)
	log.Printf("Cert subject: %#v\n", xc.Subject)
	pubk1 := xc.PublicKey

	h, t, txt, sig, _ := auth.JwtRawParse(jwt)
	log.Printf("%#v %#v\n", h, t)

	if h.Alg == "RS256" {
		rsak := pubk1.(*rsa.PublicKey)
		hasher := crypto.SHA256.New()
		hasher.Write(txt)
		hashed := hasher.Sum(nil)
		err = rsa.VerifyPKCS1v15(rsak, crypto.SHA256, hashed, sig)
		if err != nil {
			log.Println("Root Certificate not a signer")
		}
	}

}

