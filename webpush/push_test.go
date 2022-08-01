package webpush

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"testing"

	"github.com/costinm/ugate/auth"
)

/*
  RFC 8291, Appendix A: https://tools.ietf.org/html/rfc8291#appendix-A


  User agent public key (ua_public):
		BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcx aOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4

  User agent private key (ua_private):
		q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94

  Authentication secret (auth_secret):  BTBZMqHH6r4Tts7J_aSIgg

	Not used ( random in this test):

   Plaintext:  V2hlbiBJIGdyb3cgdXAsIEkgd2FudCB0byBiZSBhIHdhdGVybWVsb24
   Application server public key (as_public):
     BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8

   Application server private key (as_private):
		 yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw

   Salt:  DGv6ra1nlYgDCS1FRnbzlw


*/

func TestSendWebPush(t *testing.T) {

	privkeySub, err := base64.RawURLEncoding.DecodeString("q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94")
	if err != nil {
		t.Fatal(err)
	}
	uaPublic, err := base64.RawURLEncoding.DecodeString("BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4")
	if err != nil {
		t.Fatal(err)
	}
	authSecret, err := base64.RawURLEncoding.DecodeString("BTBZMqHH6r4Tts7J_aSIgg")
	if err != nil {
		t.Fatal(err)
	}
	//rpriv := "q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94"
	//rpub := "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"

	// Test server checks that the request is well-formed
	ts := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {

		defer request.Body.Close()

		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
		}

		message := `I am the walrus` // 15B

		// Old overhead: 2 bytes padding and 16 bytes auth tag
		// New overhead: 103 bytes
		expectedLength := len(message) + 103

		// Real: 118 (previous overhead:
		if len(body) != expectedLength {
			t.Logf("Expected body to be length %d, was %d", expectedLength, len(body))
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

		dc := auth.NewContextUA(privkeySub, uaPublic, authSecret)

		plain, err := dc.Decrypt(body)
		if err != nil {
			t.Fatal(err)
			writer.WriteHeader(502)
			return
		}

		if !bytes.Equal(plain, []byte(message)) {
			t.Error("Expected", message, "got", string(plain))
			writer.WriteHeader(501)
			return
		}

		writer.WriteHeader(201)
	}))
	defer ts.Close()

	//sub := &Subscription{ts.URL, key, a, ""}
	message := "I am the walrus"
	vapid := auth.NewAuth(nil, "", "")
	pushReq, err := NewRequest(ts.URL+"/push/", uaPublic, authSecret, message, 0, vapid)
	if err != nil {
		t.Fatal(err)
	}
	cl := ts.Client()
	//rb, _ := httputil.DumpRequest(pushReq, true)
	//log.Println(string(rb))
	res, err := cl.Do(pushReq)
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 201 {
		t.Error("Expected 201, got", res.StatusCode)
	}
	//rb, _ = httputil.DumpResponse(res, true)
	//log.Println(string(rb))
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

	//sub := &Subscription{Endpoint: ts.URL}

	vapid := auth.NewAuth(nil, "", "")
	pushReq, err := NewRequest(ts.URL+"/push/", nil, nil, "", 0, vapid)
	if err != nil {
		t.Error(err)
	}
	cl := ts.Client()
	httputil.DumpRequest(pushReq, true)
	res, err := cl.Do(pushReq)
	if err != nil {
		t.Error(err)
	}
	httputil.DumpResponse(res, true)
}

var // A fake subscription created with random key and auth values
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
