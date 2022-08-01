package ugatesvc

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/costinm/ugate"
)

func (ht *H2Transport) ForwardHTTP(w http.ResponseWriter, r *http.Request, pathH string) {
	r.Host = pathH
	r1 := CreateUpstreamRequest(w, r)

	r1.URL.Scheme = "http"

	// will be used by RoundTrip.
	r1.URL.Host = pathH

	// This uses the BTS/H2 protocol or reverse path.
	// Forward to regular sites not supported.
	res, err := ht.ug.RoundTrip(r1)
	SendBackResponse(w, r, res, err)
}

// Used by both ForwardHTTP and ForwardMesh, after RoundTrip is done.
// Will copy response headers and body
func SendBackResponse(w http.ResponseWriter, r *http.Request,
	res *http.Response, err error) {

	if err != nil {
		if res != nil {
			CopyResponseHeaders(w.Header(), res.Header)
			w.WriteHeader(res.StatusCode)
			io.Copy(w, res.Body)
			log.Println("Got ", err, res.Header)
		} else {
			http.Error(w, err.Error(), 500)
		}
		return
	}

	origBody := res.Body
	defer origBody.Close()

	CopyResponseHeaders(w.Header(), res.Header)
	w.WriteHeader(res.StatusCode)

	stats := &ugate.Stream{}
	n, err := stats.CopyBuffered(w, res.Body, true)

	log.Println("Done: ", r.URL, res.StatusCode, n, err)
}

// createUpstremRequest shallow-copies r into a new request
// that can be sent upstream.
//
// Derived from reverseproxy.go in the standard Go httputil package.
// Derived from caddy
func CreateUpstreamRequest(rw http.ResponseWriter, r *http.Request) *http.Request {
	//ctx := r.Context()
	//if cn, ok := rw.(http.CloseNotifier); ok {
	//	var cancel context.CancelFunc
	//	ctx, cancel = context.WithCancel(ctx)
	//	defer cancel()
	//	notifyChan := cn.CloseNotify()
	//	go func() {
	//		select {
	//		case <-notifyChan:
	//			cancel()
	//		case <-ctx.Done():
	//		}
	//	}()
	//}
	ctx := context.Background()

	// URL, Form, TransferEncoding, Header, Trailer, URL
	outreq := r.Clone(ctx)

	// We should set body to nil explicitly if request body is empty.
	// For DmDns requests the Request Body is always non-nil.
	if r.ContentLength == 0 {
		outreq.Body = nil
	}
	if outreq.Header == nil {
		outreq.Header = make(http.Header) // Issue 33142: historical behavior was to always allocate
	}
	outreq.Close = false

	// We are modifying the same underlying map from req (shallow
	// copied above) so we only copy it if necessary.
	copiedHeaders := false

	// Remove hop-by-hop headers listed in the "Connection" header.
	// See RFC 2616, section 14.10.
	if c := outreq.Header.Get("Connection"); c != "" {
		for _, f := range strings.Split(c, ",") {
			if f = strings.TrimSpace(f); f != "" {
				if !copiedHeaders {
					outreq.Header = make(http.Header)
					CopyRequestHeaders(outreq.Header, r.Header)
					copiedHeaders = true
				}
				outreq.Header.Del(f)
			}
		}
	}

	// Remove hop-by-hop headers to the backend. Especially
	// important is "Connection" because we want a persistent
	// connection, regardless of what the client sent to us.
	for _, h := range hopHeaders {
		if outreq.Header.Get(h) != "" {
			if !copiedHeaders {
				outreq.Header = make(http.Header)
				CopyRequestHeaders(outreq.Header, r.Header)
				copiedHeaders = true
			}
			outreq.Header.Del(h)
		}
	}

	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		// If we aren't the first proxy, retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := outreq.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		outreq.Header.Set("X-Forwarded-For", clientIP)
	}

	return outreq
}

// Hop-by-hop headers. These are removed when sent to the backend in createUpstreamRequest
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Alt-Svc",
	"Alternate-Protocol",
	"Connection",
	"Keep-Alive",
	"HTTPGate-Authenticate",
	"HTTPGate-Authorization",
	"HTTPGate-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
	"Te",                  // canonicalized version of "TE"
	"Trailer",             // not Trailers per URL above; http://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",
}

// used in createUpstreamRequetst to copy the headers to the new req.
func CopyRequestHeaders(dst, src http.Header) {
	for k, vv := range src {
		if _, ok := dst[k]; ok {
			// skip some predefined headers
			// see https://github.com/mholt/caddy/issues/1086
			if _, shouldSkip := skipHeaders[k]; shouldSkip {
				continue
			}
			// otherwise, overwrite to avoid duplicated fields that can be
			// problematic (see issue #1086) -- however, allow duplicate
			// Server fields so we can see the reality of the proxying.
			if k != "Server" {
				dst.Del(k)
			}
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// Also used in httpproxy_capture, for forward http proxy
func CopyResponseHeaders(dst, src http.Header) {
	for k := range dst {
		dst.Del(k)
	}
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

// skip these headers if they already exist.
// see https://github.com/mholt/caddy/pull/1112#discussion_r80092582
var skipHeaders = map[string]struct{}{
	"Content-Type":        {},
	"Content-Disposition": {},
	"accept-Ranges":       {},
	"Set-Cookie":          {},
	"Cache-Control":       {},
	"Expires":             {},
}
