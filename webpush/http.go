package webpush

import (
	"crypto/rand"
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/costinm/meshauth"
	"github.com/costinm/ugate/auth"
)

type Backoff interface {
	BackoffSleep()
	BackoffReset()
}

var ReceiveBaseUrl = "https://127.0.0.1:5228/"

// HTTPHandlerSend can send regular messages.
// Can be exposed without auth on 127.0.0.1, or use normal
// auth.
//
// Mapped to /s/[DESTID]/topic?...
//
// q or path can be used to pass command. Body and query string are sent.
// TODO: compatibility with cloud events and webpush
// TODO: RBAC (including admin check for system notifications)
func (mux *Mux) HTTPHandlerSend(w http.ResponseWriter, r *http.Request) {
	//transport.GetPeerCertBytes(r) or auth context
	r.ParseForm()

	var cmd string
	var parts []string
	q := r.Form.Get("q")

	if q != "" {
		parts = strings.Split(q, "/")
		cmd = q
	} else {
		parts = strings.Split(r.URL.Path, "/")
		parts = parts[2:]
		cmd = strings.Join(parts, " ")

		log.Println("MSG_SEND: ", parts, "--", cmd)
	}

	params := map[string]string{}
	for k, v := range r.Form {
		params[k] = v[0]
	}
	var err error
	var body []byte
	if r.Method == "POST" {
		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			return
		}
	}

	msg := NewMessage(cmd, params).SetDataJSON(body)
	err = mux.SendMessage(msg)
	if err == nil {
		w.WriteHeader(200)
		return
	}
	w.WriteHeader(500)
	w.Write([]byte(err.Error()))
}

var SharedWPAuth = []byte{1}

// Webpush handler - on /push[/VIP]/topic[?params], on the HTTPS handler
//
// Auth: VAPID or client cert - results in VIP of sender
func (mux *Mux) HTTPHandlerWebpush(w http.ResponseWriter, r *http.Request) {
	// VAPID or client cert already authenticated.
	rctx := auth.AuthContext(r.Context())

	parts := strings.Split(r.RequestURI, "/")
	if len(parts) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid URL"))
		return
	}

	log.Println("WEBPUSH over HTTP ", parts)

	dest := parts[2]
	if dest == "" || dest == mux.Auth.Name || dest == mux.Auth.Self() {
		ec := meshauth.NewContextUA(mux.Auth.Priv, mux.Auth.PublicKey, SharedWPAuth)
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Body error"))
			return
		}

		msgb, err := ec.Decrypt(b)
		if err != nil {
			log.Println("Decrypt error ", err)
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Decrypt error"))
			return
		}

		params := map[string]string{}
		for k, v := range r.Form {
			params[k] = v[0]
		}

		ev := NewMessage("."+strings.Join(parts[3:], "/"), params).SetDataJSON(msgb)

		//ev := mux.ProcessMessage(msgb, rctx)
		log.Println("GOT WEBPUSH: ", rctx.ID(), string(msgb), ev)

		if ev == nil {
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Invalid format"))
				return
			}
		}

		role := rctx.Role
		if role == "" || role == "guest" {
			log.Println("Unauthorized ")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Unauthorized"))
			return
		}
		ev.From = rctx.ID()

		mux.HandleMessageForNode(ev)
	} else {
		// Dest is remote, we're just forwarding.

	}

	w.WriteHeader(201)
}

// MonitorEvents will connect to a mesh address and monitor the messages.
//
// base is used for forwarding.
//
//func (w *Mux) MonitorEvents(node Backoff, idhex string, path []string) {
//	hc := transport.NewSocksHttpClient("")
//	hc.Timeout = 1 * time.Hour
//
//	if idhex == "" {
//		hc = http.DefaultClient
//	}
//
//	for {
//		t0 := time.Now()
//
//		err := w.MonitorNode(hc, idhex, path)
//		if err != nil {
//			log.Println("WATCH_ERR", idhex, err, time.Since(t0))
//			node.BackoffSleep()
//			continue
//		}
//		node.BackoffReset()
//
//		log.Println("WATCH_CLOSE", idhex, time.Since(t0))
//		node.BackoffSleep()
//	}
//
//}

// UA represents a "user agent" - or client using the webpush protocol
type UA struct {
	// URL of the subscribe for the push service
	PushService string
}

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

// Create a subscription, using the Webpush standard protocol.
//
// URL is "/subscribe", no header required ( but passing a VAPID or mtls),
// response in 'location' for read and Link for sub endpoint.
func (ua *UA) Subscribe() (sub *Subscription, err error) {
	res, err := http.Post(ua.PushService+"/subscribe", "text/plain", nil)

	if err != nil {
		return
	}
	sub = &Subscription{}
	sub.Location = res.Header.Get("location")
	links := textproto.MIMEHeader(res.Header)["Link"]
	for _, l := range links {
		for _, link := range strings.Split(l, ",") {
			parts := strings.Split(link, ";")
			if len(parts) > 1 &&
				strings.TrimSpace(parts[1]) == "rel=\"urn:ietf:params:push\"" {
				sub.Endpoint = parts[0]
			}
		}
	}

	// generate encryption key and authenticator
	return
}

// Subscribe creates a subscription. Initial version is just a
// random - some interface will be added later, to allow sets.
func (mux *Mux) SubscribeHandler(res http.ResponseWriter, req *http.Request) {
	// For simple testing we ignore sender auth, as well as subscription sets
	token := make([]byte, 16)
	rand.Read(token)

	id := base64.RawURLEncoding.EncodeToString(token)

	res.WriteHeader(201)

	// TODO: try to use a different server, to verify UA is
	// parsing both

	// Used for send - on same server as subscribe
	res.Header().Add("Link", "</p/"+
		id+
		">;rel=\"urn:ietf:params:push\"")

	// May provide support for set: should be enabled if a
	// set interface is present, want to test without set as well
	//res.Header().StartListener("Link", "</p/" +
	//	"JzLQ3raZJfFBR0aqvOMsLrt54w4rJUsV" +
	//	">;rel=\"urn:ietf:params:push:set\"")

	res.Header().Add("Location", ReceiveBaseUrl+"/r/"+id)

	return
}
