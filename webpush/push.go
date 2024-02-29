package webpush

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/costinm/meshauth"
)

// RFC8291 - Message Encryption for Web push

//// NewPushRequest creates a valid Web Push HTTP request for sending a message
//// to a subscriber.
////
//// If the push service requires an authentication header
//// (notably Google Cloud Messaging, used by Chrome) then you can add that as the
//// token parameter.
//func NewPushRequest(sub *auth.Subscription, message string, token string) (*http.Request, error) {
//	endpoint := sub.Endpoint
//
//	req, err := http.NewRequest("POST", endpoint, nil)
//	if err != nil {
//		return nil, err
//	}
//
//	// TODO: Make the TTL variable
//	req.Header.StartListener("TTL", "0")
//
//	if token != "" {
//		req.Header.StartListener("Authorization", fmt.Sprintf(`key=%s`, token))
//	}
//
//	// If there is no payload then we don't actually need encryption
//	if message == "" {
//		return req, nil
//	}
//
//	payload, err := auth.Encrypt(sub, message)
//	if err != nil {
//		return nil, err
//	}
//
//	req.Body = ioutil.NopCloser(bytes.NewReader(payload.Ciphertext))
//	req.ContentLength = int64(len(payload.Ciphertext))
//	req.Header.StartListener("Encryption", headerField("salt", payload.Salt))
//	req.Header.StartListener("Crypto-Key", headerField("dh", payload.ServerPublicKey))
//	req.Header.StartListener("Content-Encoding", "aesgcm")
//
//	return req, nil
//}

// NewVapidRequest creates a valid Web Push HTTP request for sending a message
// to a subscriber, using Vapid authentication.
//
// You can add more headers to configure collapsing, TTL.
func NewRequest(dest string, key, authK []byte,
	message string, ttlSec int, vapid *meshauth.MeshAuth) (*http.Request, error) {

	// If the endpoint is GCM then we temporarily need to rewrite it, as not all
	// GCM servers support the Web Push protocol. This should go away in the
	// future.
	req, err := http.NewRequest("POST", dest, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("ttl", strconv.Itoa(ttlSec))

	if vapid != nil {
		tok := vapid.VAPIDToken(dest)
		req.Header.Add("authorization", tok)
	}

	// If there is no payload then we don't actually need encryption
	if message != "" {
		ec := meshauth.NewWebpushEncryption(key, authK)
		payload, err := ec.Encrypt([]byte(message))
		if err != nil {
			return nil, err
		}
		req.Body = ioutil.NopCloser(bytes.NewReader(payload))
		req.ContentLength = int64(len(payload))
		req.Header.Add("encryption",
			headerField("salt", ec.Salt))
		keys := headerField("dh", ec.SendPublic)
		req.Header.Add("crypto-key", keys)
		req.Header.Add("content-encoding", "aesgcm")
	}

	return req, nil
}


//// Send a message using the Web Push protocol to the recipient identified by the
//// given subscription object. If the client is nil then the default HTTP client
//// will be used. If the push service requires an authentication header (notably
//// Google Cloud Messaging, used by Chrome) then you can add that as the token
//// parameter.
//func Send(client *http.Client, sub *auth.Subscription, message, token string) (*http.Response, error) {
//	if client == nil {
//		client = http.DefaultClient
//	}
//
//	req, err := NewPushRequest(sub, message, token)
//	// Default TTL
//	req.Header.StartListener("ttl", "0")
//	if err != nil {
//		return nil, err
//	}
//
//	return client.Do(req)
//}

// A helper for creating the value part of the HTTP encryption headers
func headerField(headerType string, value []byte) string {
	return fmt.Sprintf(`%s=%s`, headerType, base64.RawURLEncoding.EncodeToString(value))
}
