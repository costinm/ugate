// Copyright 2016 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ugate

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"
)

// RFC9292 - VAPID

var (
	// encoded {"typ":"JWT","alg":"ES256"}
	vapidPrefix = []byte("eyJ0eXAiOiJKV1QiLCJhbGciOiJFUzI1NiJ9.")
	dot         = []byte(".")
)

type jwtHead struct {
	Typ string `json:"typ"`
	Alg string `json:"alg,omitempty"`
}

type JWT struct {
	//An "aud" (Audience) claim in the token MUST include the Unicode
	//serialization of the origin (Section 6.1 of [RFC6454]) of the push
	//resource URL.  This binds the token to a specific push service and
	//ensures that the token is reusable for all push resource URLs that
	//share the same origin.
	Aud string `json:"aud"`

	//If the application server wishes to provide contact details, it MAY
	//include a "sub" (Subject) claim in the JWT.  The "sub" claim SHOULD
	//include a contact URI for the application server as either a
	//"mailto:" (email) [RFC6068] or an "https:" [RFC2818] URI.
	Sub string `json:"sub,omitempty"`

	// Max 24h
	Exp int64 `json:"exp"`
}

// VAPIDToken creates a token with the specified endpoint, using configured Sub id
// and a default expiration (1h).
//
// The optional (unauthenticated) Sub field is populated from Name@Domain or Domain.
// The DMesh VIP is based on the public key of the signer.
// AUD is the URL from the subscription - for DMesh https://VIP:5228/s or
// https://DOMAIN:5228/s
func (auth *Auth) VAPIDToken(aud string) string {
	jwt := JWT{}
	u, err := url.Parse(aud)
	if err != nil || len(u.Host) == 0 {
		jwt.Aud = aud
	} else {
		jwt.Aud = "https://" + u.Host
	}
	if auth.Domain != "" {
		jwt.Sub = auth.Domain
		if auth.Name != "" {
			jwt.Sub = auth.Name + "@" + auth.Domain
		}
	}
	jwt.Exp = time.Now().Unix() + 3600
	t, _ := json.Marshal(jwt)

	enc := base64.RawURLEncoding
	// Base64URL for the content of the token
	t64 := make([]byte, enc.EncodedLen(len(t)))
	enc.Encode(t64, t)

	token := make([]byte, len(t)+len(vapidPrefix)+100)
	token = append(token[:0], vapidPrefix...)
	token = append(token, t64...)

	hasher := crypto.SHA256.New()
	hasher.Write(token)

	if r, s, err := ecdsa.Sign(rand.Reader, auth.EC256PrivateKey, hasher.Sum(nil)); err == nil {
		// Vapid key is 32 bytes
		keyBytes := 32
		sig := make([]byte, 2*keyBytes)

		rBytes := r.Bytes()
		rBytesPadded := make([]byte, keyBytes)
		copy(rBytesPadded[keyBytes-len(rBytes):], rBytes)

		sBytes := s.Bytes()
		sBytesPadded := make([]byte, keyBytes)
		copy(sBytesPadded[keyBytes-len(sBytes):], sBytes)

		sig = append(sig[:0], rBytesPadded...)
		sig = append(sig, sBytesPadded...)

		sigB64 := make([]byte, enc.EncodedLen(len(sig)))
		enc.Encode(sigB64, sig)

		token = append(token, dot...)
		token = append(token, sigB64...)
	}

	return "vapid t=" + string(token) + ", k=" + auth.pub64
}

// CheckVAPID verifies the signature and returns the token and public key.
// expCheck should be set to current time to set expiration
func CheckVAPID(tok string, now time.Time) (jwt *JWT, pub []byte, err error) {
	// Istio uses oidc - will make a HTTP request to fetch the .well-known from
	// iss.
	// provider, err := oidc.NewProvider(context.Background(), iss)
	// Provider uses verifier, using KeySet interface 'verifySignature(jwt)
	// The keyset is expected to be cached and configured (trusted)

	scheme, keys := ParseAuth(tok)
	if scheme != "vapid" {
		return nil, nil, errors.New("Unexected scheme " + scheme)
	}

	tok = keys["t"]
	pubk := keys["k"]

	// Token is parsed with square/go-jose
	parts := strings.Split(tok, ".")
	if len(parts) < 2 {
		return nil, nil, fmt.Errorf("VAPID: malformed jwt, parts=%d", len(parts))
	}
	head, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("VAPI: malformed jwt %v", err)
	}
	h := &jwtHead{}
	json.Unmarshal(head, h)

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("VAPI: malformed jwt %v", err)
	}
	b := &JWT{}
	json.Unmarshal(payload, b)

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, nil, fmt.Errorf("VAPI: malformed jwt %v", err)
	}

	publicUncomp, err := base64.RawURLEncoding.DecodeString(pubk)
	if err != nil {
		return nil, nil, fmt.Errorf("VAPI: malformed jwt %v", err)
	}
	if len(publicUncomp) != 65 {
		return nil, nil, fmt.Errorf("VAPI: malformed jwt %d", len(pubk))
	}

	// TODO: Check expiration !

	hasher := crypto.SHA256.New()
	hasher.Write([]byte(tok[0 : len(parts[0])+len(parts[1])+1]))

	x, y := elliptic.Unmarshal(elliptic.P256(), publicUncomp)
	pk := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

	r := big.NewInt(0).SetBytes(sig[0:32])
	s := big.NewInt(0).SetBytes(sig[32:64])
	match := ecdsa.Verify(pk, hasher.Sum(nil), r, s)

	if !match {
		return nil, nil, errors.New("invalid signature")
	}

	if !now.IsZero() {
		expT := time.Unix(b.Exp, 0)
		if expT.Before(now) {
			return nil, nil, errors.New("Expired token")
		}

	}

	return b, publicUncomp, nil
}

// ParseAuth splits the Authorization header, returning the scheme and parameters.
func ParseAuth(auth string) (string, map[string]string) {
	auth = strings.TrimSpace(auth)
	params := map[string]string{}

	spaceIdx := strings.Index(auth, " ")
	if spaceIdx == -1 {
		return auth, params
	}

	scheme := auth[0:spaceIdx]
	auth = auth[spaceIdx:]

	pl := strings.Split(auth, ",")
	for _, p := range pl {
		p = strings.Trim(p, " ")
		kv := strings.Split(p, "=")
		if len(kv) == 2 {
			key := strings.Trim(kv[0], " ")
			params[key] = kv[1]
		}
	}

	return scheme, params
}
