package ugatesvc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/costinm/hbone/nio"
	"golang.org/x/net/http2"
)

// Dedicated BTS handler, for accepted connections with TLS.
//
// Port 443 (if root or redirected), or BASE + 7
//
// curl https://$NAME/ --connect-to $NAME:443:127.0.0.1:15007
func (ug *UGate) acceptedHbone(rawStream *nio.Stream) error {

	tlsCfg := ug.TLSConfig
	tc, err := ug.NewTLSConnIn(rawStream.Context(), rawStream.Listener, nil, rawStream, tlsCfg)
	if err != nil {
		rawStream.ReadErr = err
		log.Println("TLS: ", rawStream.RemoteAddr(), rawStream.Dest, err)
		return nil
	}

	// Handshake done. Now we have access to the ALPN.
	tc.PostDial(tc, nil)

	// http2 and http expect a net.Listener, and do their own accept()
	ug.H2Handler.h2Server.ServeConn(
		tc,
		&http2.ServeConnOpts{
			Handler: ug.H2Handler, // Also plain text, needs to be upgraded
			Context: tc.Context(), // associated with the stream, with cancel

			//Context: // can be used to cancel, pass meta.
			// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
		})
	return nil
}

// Handles an accepted connection with plain text h2, intended for
// hbone protocol.
func (ug *UGate) acceptedHboneC(tc *nio.Stream) error {
	tc.PostDial(tc, nil)
	ug.H2Handler.h2Server.ServeConn(
		tc,
		&http2.ServeConnOpts{
			Handler: http.HandlerFunc(ug.H2Handler.httpHandleHboneC), // Also plain text, needs to be upgraded
			Context: tc.Context(),                                    // associated with the stream, with cancel
			//Context: // can be used to cancel, pass meta.
			// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
		})
	return nil
}

func (l *H2Transport) httpHandleHboneC(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	defer func() {
		// TODO: add it to an event buffer
		l.ug.OnHClose("Hbone", "", "", r, time.Since(t0))

		if r := recover(); r != nil {
			fmt.Println("Recovered in hbone", r)

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

	// TODO: parse Envoy / hbone headers.

	if nio.DebugClose {
		log.Println("Hbone-RoundTripStart ", r.Method, r.URL, r.Proto, r.Header, RemoteID, "", r.RemoteAddr)
	}

	// TCP proxy for SSH
	if r.Method == "CONNECT" || strings.HasPrefix(r.RequestURI, "/_hbone/") {
		l.ug.HandleTCPProxy(w, r)
	}

	tlsCfg := l.ug.TLSConfig

	// Create a stream, used for proxy with caching.
	rawStream := nio.NewStreamRequest(r, w, nil)

	tc, err := l.ug.NewTLSConnIn(rawStream.Context(), rawStream.Listener, []string{"h2"}, rawStream,
		tlsCfg)
	if err != nil {
		rawStream.ReadErr = err
		log.Println("TLS: ", rawStream.RemoteAddr(), rawStream.Dest, err)
		return
	}

	// TODO: All Istio checks go here. The TLS handshake doesn't check
	// root cert or anything - this is proof of concept only, to eval
	// perf.

	if tc.TLS.NegotiatedProtocol == "h2" {
		// http2 and http expect a net.Listener, and do their own accept()
		l.ug.H2Handler.h2Server.ServeConn(
			tc,
			&http2.ServeConnOpts{
				Handler: http.HandlerFunc(l.ug.H2Handler.httpHandleHboneCHTTP),
				Context: tc.Context(), // associated with the stream, with cancel
			})
	} else {
		// HTTP/1.1
		// TODO. Typically we want to upgrade over the wire to H2
	}
}

// At this point we got the TLS stream over H2, and forward to the app
// We still need to know if the app is H2C or HTTP/1.1
func (l *H2Transport) httpHandleHboneCHTTP(w http.ResponseWriter, r *http.Request) {
	l.ForwardHTTP(w, r, "127.0.0.1:8080")
}

// HboneCat copies stdin/stdout to a HBONE stream.
func HboneCat(ug *UGate, urlOrHost string, tls bool, stdin io.ReadCloser,
	stdout io.WriteCloser) error {
	i, o := io.Pipe()

	if !strings.HasPrefix(urlOrHost, "https://") {
		h, p, err := net.SplitHostPort(urlOrHost)
		if err != nil {
			return err
		}
		urlOrHost = "https://" + h + "/hbone/:" + p
	}
	r, _ := http.NewRequest("POST", urlOrHost, i)
	res, err := ug.RoundTrip(r)
	if err != nil {
		return err
	}

	var nc *nio.Stream
	if tls {
		plain := nio.NewStreamRequestOut(r, o, res, nil)
		nc, err = ug.NewTLSConnOut(context.Background(), plain, ug.TLSConfig,
			"", []string{"h2"})
		if err != nil {
			return err
		}
	} else {
		nc = nio.NewStreamRequestOut(r, o, res, nil)
	}
	go func() {
		b1 := make([]byte, 1024)
		for {
			n, err := nc.Read(b1)
			if err != nil {
				stdout.Close()
				stdin.Close()
				log.Println("Tun read err", err)
				return
			}
			stdout.Write(b1[0:n])
		}
	}()

	b1 := make([]byte, 1024)
	for {
		n, err := stdin.Read(b1)
		if err != nil {
			return err
		}
		nc.Write(b1[0:n])
	}
	return nil
}
