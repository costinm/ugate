package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// Subscription holds the useful values from a PushSubscription object acquired
// from the browser.
//
// https://w3c.github.io/push-api/
//
// Returned as result of /subscribe
type Subscription struct {
	// Endpoint is the URL to send the Web Push message to. Comes from the
	// endpoint field of the PushSubscription.
	Endpoint string

	// Key is the client's public key. From the getKey("p256dh") or keys.p256dh field.
	Key []byte

	// Auth is a value used by the client to validate the encryption. From the
	// keys.auth field.
	// The encrypted aes128gcm will have 16 bytes authentication tag derived from this.
	// This is the pre-shared authentication secret.
	Auth []byte

	// Used by the UA to receive messages, as PUSH promises
	Location string
}

// SubscriptionFromJSON is a convenience function that takes a JSON encoded
// PushSubscription object acquired from the browser and returns a pointer to a
// node.
func SubscriptionFromJSON(b []byte) (*Subscription, error) {
	var sub struct {
		Endpoint string
		Keys     struct {
			P256dh string
			Auth   string
		}
	}
	if err := json.Unmarshal(b, &sub); err != nil {
		return nil, err
	}

	b64 := base64.URLEncoding.WithPadding(base64.NoPadding)

	// Chrome < 52 incorrectly adds padding when Base64 encoding the values, so
	// we need to strip that out
	key, err := b64.DecodeString(strings.TrimRight(sub.Keys.P256dh, "="))
	if err != nil {
		return nil, err
	}

	auth, err := b64.DecodeString(strings.TrimRight(sub.Keys.Auth, "="))
	if err != nil {
		return nil, err
	}

	return &Subscription{sub.Endpoint, key, auth, ""}, nil
}


