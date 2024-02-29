package httpwrapper

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"log/slog"

	"github.com/costinm/meshauth"
	"github.com/costinm/ssh-mesh/nio"
)

// Deprecated - replaced by  meshauth and otel
// This wraps a http call with authz.

type HttpHandler struct {
	Handler http.Handler
	Logger  *slog.Logger
}

// Common entry point for H1, H2 - both plain and tls
// Will do the 'common' operations - authn, authz, logging, metrics for all BTS and regular HTTP.
//
// Important:
// When using for BTS we need to work around golang http stack implementation.
// This should be used as fallback - QUIC and WebRTC have proper mux and TUN support.
// In particular, while H2 POST and CONNECT allow req/res Body to act as TCP stream,
// the closing (FIN/RST) are very tricky:
//   - ResponseWriter (in BTS server) does not have a 'Close' method, it is closed after
//     the method returns. That means we can't signal the TCP FIN or RST, which breaks some
//     protocols.
//   - The request must be fully consumed before the method returns.
func (l *HttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	var RemoteID string
	var SAN string

	defer func() {
		// TODO: add it to an event buffer
		if l.Logger != nil {
			l.Logger.Log(r.Context(), slog.LevelInfo, "remoteID", RemoteID,
				"SAN", SAN, "request", r.URL, "time", time.Since(t0))
		}
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)

			debug.PrintStack()

			// find out exactly what the error was and set err
			var err error

			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}
			if err != nil {
				fmt.Println("ERRROR: ", err)
			}
		}
	}()

	vapidH := r.Header["Authorization"]
	if len(vapidH) > 0 {
		tok, pub, err := meshauth.CheckVAPID(vapidH[0], time.Now())
		if err == nil {
			RemoteID = meshauth.IDFromPublicKeyBytes(pub)
			SAN = tok.Sub
		}
	}

	tls := r.TLS
	// If the request was handled by normal uGate listener.
	us := r.Context().Value("util.Stream")
	if ugs, ok := us.(nio.Stream); ok {
		tls = ugs.TLSConnectionState()
		r.TLS = tls
	}
	// other keys:
	// - http-server (*http.Server)
	// - local-addr - *net.TCPAddr
	//

	if tls != nil && len(tls.PeerCertificates) > 0 {
		pk1 := tls.PeerCertificates[0].PublicKey
		RemoteID = meshauth.PublicKeyBase32SHA(pk1)
		// TODO: Istio-style, signed by a trusted CA. This is also for SSH-with-cert
		sans, _ := meshauth.GetSAN(tls.PeerCertificates[0])
		if len(sans) > 0 {
			SAN = sans[0]
		}
	}
	// Using the 'from' header internally
	if RemoteID != "" {
		r.Header.Set("from", RemoteID)
	} else {
		r.Header.Del("from")
	}

	l.Handler.ServeHTTP(w, r)
}
