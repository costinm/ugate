package cmd

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/costinm/meshauth/pkg/tokens"
	k8sc "github.com/costinm/mk8s"

	"github.com/costinm/meshauth"
)

type DecodeTokenRequest struct {
	Token string
}

func (r *DecodeTokenRequest) Run(ctx context.Context) error {
	fmt.Println(tokens.TokenPayload(r.Token))

	return nil
}

// Decode a JWT.
// If crt is specified - verify it using that cert
// Examples:
// {"aud":["https://container.googleapis.com/v1/projects/costin-asm1/locations/us-west1-c/clusters/ip6"],
//
//	"exp":1749000048,
//	 "iat":1717464048,
//	 "iss":"https://container.googleapis.com/v1/projects/costin-asm1/locations/us-west1-c/clusters/ip6",
//	"kubernetes.io":{
//	     "namespace":"dev",
//	    "pod":{
//	       "name":"dev-5b6dc547f7-fx7t6",
//	       "uid":"f4908906-05ea-4f28-8954-884b8468d686"},
//	     "serviceaccount":{
//	        "name":"dev",
//	        "uid":"b7f67f53-94a7-4739-ada8-30c088d6f535"},
//	     "warnafter":1717467655},
//	  "nbf":1717464048,
//	  "sub":"system:serviceaccount:dev:dev"}
//
//	{"aud":["costin-asm1.svc.id.goog"],...}
func decodeJWT(jwt string) {
	// TODO: verify if it's a VAPID
	parts := strings.Split(jwt, ".")
	p1b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println(string(p1b))
}

func decodeChain(jwt string) {
	scrt, _ := ioutil.ReadFile("server.crt")
	block, _ := pem.Decode(scrt)
	xc, _ := x509.ParseCertificate(block.Bytes)
	log.Printf("Cert subject: %#v\n", xc.Subject)
	pubk1 := xc.PublicKey

	h, t, txt, sig, _ := meshauth.JwtRawParse(jwt)
	log.Printf("%#v %#v\n", h, t)

	if h.Alg == "RS256" {
		rsak := pubk1.(*rsa.PublicKey)
		hasher := crypto.SHA256.New()
		hasher.Write(txt)
		hashed := hasher.Sum(nil)
		err := rsa.VerifyPKCS1v15(rsak, crypto.SHA256, hashed, sig)
		if err != nil {
			log.Println("Root Certificate not a signer")
		}
	}
}

type TokenRequest struct {
	Aud       string
	KSA       string
	Namespace string

	Fed   bool
	GCPSA string
}

func (r *TokenRequest) Run(ctx context.Context) (string, error) {
	var err error

	k := k8sc.Default()

	def := k.Default
	if def == nil {
		return "", err
	}
	projectID, _, _ := def.GcpInfo()

	var tokenProvider meshauth.TokenSource

	def = def.RunAs(r.Namespace, r.KSA)

	if r.Fed {
		tokenProvider = tokens.NewFederatedTokenSource(&tokens.STSAuthConfig{
			AudienceSource: projectID + ".svc.id.goog",
			TokenSource:    def,
		})
	} else if r.GCPSA == "" {
		tokenProvider = def // .NewK8STokenSource(*aud)
	} else {
		gsa := r.GCPSA
		tokenProvider = tokens.NewFederatedTokenSource(&tokens.STSAuthConfig{
			AudienceSource: projectID + ".svc.id.goog",
			TokenSource:    def,
			GSA:            gsa,
		})
	}
	if err != nil {
		return "", err
	}

	t, err := tokenProvider.GetToken(ctx, r.Aud)

	if err != nil {
		return "", err
	}
	return t, nil
}
