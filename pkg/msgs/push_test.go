package msgs

import (
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendWebPush(t *testing.T) {
	// Test server checks that the request is well-formed
	ts := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)

		defer request.Body.Close()

		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
		}

		message := `I am the walrus`

		// 2 bytes padding and 16 bytes auth tag
		expectedLength := len(message) + 2 + 16

		if len(body) != expectedLength {
			t.Errorf("Expected body to be length %d, was %d", expectedLength, len(body))
		}

		if request.Header.Get("TTL") == "" {
			t.Error("Expected TTL header to be set")
		}

		if request.Header.Get("Content-Encoding") != "aesgcm" {
			t.Errorf("Expected Content-Encoding header to be aesgcm, got %v", request.Header.Get("Content-Encoding"))
		}

		if !strings.HasPrefix(request.Header.Get("Crypto-Key"), "dh=") {
			t.Errorf("Expected Crypto-Key header to have a dh field, got %v", request.Header.Get("Crypto-Key"))
		}

		if !strings.HasPrefix(request.Header.Get("Encryption"), "salt=") {
			t.Errorf("Expected Encryption header to have a salt field, got %v", request.Header.Get("Encryption"))
		}
	}))
	defer ts.Close()

	key, err := base64.URLEncoding.DecodeString("BCXJI0VW7evda9ldlo18MuHhgQVxWbd0dGmUfpQedaD7KDjB8sGWX5iiP7lkjxi-A02b8Fi3BMWWLoo3b4Tdl-c=")
	if err != nil {
		t.Error(err)
	}
	a, err := base64.URLEncoding.DecodeString("WPF9D0bTVZCV2pXSgj6Zug==")
	if err != nil {
		t.Error(err)
	}

	sub := &Subscription{ts.URL, key, a, ""}
	message := "I am the walrus"

	if _, err = Send(nil, sub, message, ""); err != nil {
		t.Error(err)
	}
}

func TestSendTickle(t *testing.T) {
	// Test server checks that the request is well-formed
	ts := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)

		defer request.Body.Close()

		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
		}

		if len(body) != 0 {
			t.Errorf("Expected body to be length 0, was %d", len(body))
		}

		if request.Header.Get("TTL") == "" {
			t.Error("Expected TTL header to be set")
		}
	}))
	defer ts.Close()

	sub := &Subscription{Endpoint: ts.URL}

	if _, err := Send(nil, sub, "", ""); err != nil {
		t.Error(err)
	}
}

var 	// A fake subscription created with random key and auth values
	subscriptionJSON = []byte(`{
		"endpoint": "https://example.com/",
		"keys": {
			"p256dh": "BCXJI0VW7evda9ldlo18MuHhgQVxWbd0dGmUfpQedaD7KDjB8sGWX5iiP7lkjxi-A02b8Fi3BMWWLoo3b4Tdl-c=",
			"auth": "WPF9D0bTVZCV2pXSgj6Zug=="
		}
	}`)


func TestSubscriptionFromJSON(t *testing.T) {
	_, err := SubscriptionFromJSON(subscriptionJSON)
	if err != nil {
		t.Errorf("Failed to parse main sample subscription: %v", err)
	}

	// key and auth values are valid Base64 with or without padding
	_, err = SubscriptionFromJSON([]byte(`{
		"endpoint": "https://example.com",
		"keys": {
			"p256dh": "AAAA",
			"auth": "AAAA"
		}
	}`))
	if err != nil {
		t.Errorf("Failed to parse subscription with 4-character values: %v", err)
	}

	// key and auth values are padded Base64
	_, err = SubscriptionFromJSON([]byte(`{
		"endpoint": "https://example.com",
		"keys": {
			"p256dh": "AA==",
			"auth": "AAA="
		}
	}`))
	if err != nil {
		t.Errorf("Failed to parse subscription with padded values: %v", err)
	}

	// key and auth values are unpadded Base64
	_, err = SubscriptionFromJSON([]byte(`{
		"endpoint": "https://example.com",
		"keys": {
			"p256dh": "AA",
			"auth": "AAA"
		}
	}`))
	if err != nil {
		t.Errorf("Failed to parse subscription with unpadded values: %v", err)
	}
}

