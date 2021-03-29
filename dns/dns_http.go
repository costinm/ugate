//+build DNS_HTTP

package dns

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/miekg/dns"
)

//mux.Handle("/dns/", s)

var (
	debugHttp = os.Getenv("DEBUG_DNS") == "1"
)

// DNS over h2: draft-ietf-doh-dns-over-https
// dns query param, base64url encoded request.
// response with application/dns-message type, binary response

// https://dns.google.com/resolve?
// name
// type (default 1 - A)
// cd=1 - disable DNSSEC validation
// edns_client_subnet - for geo
// Response: json,
// Status, Answer [ { name, type, data, ttl} ]

// example:
//

func sendRes(w http.ResponseWriter, res *dns.Msg, r *http.Request, m []byte) {
	if debugHttp {
		log.Printf("DNS-HTTP-Res: %v %s %d", r.RemoteAddr, res.Question[0].Name, len(res.Answer))
	}
	ttl := uint32(0)
	for _, a := range res.Answer {
		t := a.Header().Ttl
		if t < ttl {
			ttl = t
		}
	}
	//ServerMetrics.Total.Add(1)
	w.Header().Add("cache-control", "max-age="+strconv.Itoa(int(ttl)))

	resB, _ := res.PackBuffer(m)
	w.Write(m[0:len(resB)])
}

func (s *DmDns) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//t0 := time.Now()
	m := bufferPoolHttp.Get().([]byte)
	m = m[0:cap(m)]
	defer bufferPoolHttp.Put(m)

	req := new(dns.Msg)
	w.Header().Add("content-type", "application/dns-message")

	var n int
	var err error
	if r.Method == http.MethodGet {
		r.ParseForm()
		dnsQ := r.Form["dns"]
		if len(dnsQ) > 0 && len(dnsQ[0]) > 0 {
			n, err = base64.RawURLEncoding.Decode(m, []byte(dnsQ[0]))
			if err != nil {
				return
			}
		}
	} else if r.Method == http.MethodPost {
		n, err = io.ReadFull(r.Body, m)
		if err != nil {
			return
		}
	} else {
		return
	}
	err = req.Unpack(m[0:n])
	if err != nil {
		log.Print("Invalid request ", err)
		return
	}

	req.Id = dns.Id()

	res := s.Process(req)

	sendRes(w, res, r, m)
	if len(res.Answer) > 0 {
		//log.Println("HTTP DNS ", req.Question[0].Name, time.Since(t0))
		//ServerMetrics.Latency.Add(time.Since(t0).Seconds())
	}
	return
}

// ForwardHttp forwards the req to a http server, using dmesh-specific DNS-over-HTTP
// Using GET method - see https://developers.cloudflare.com/1.1.1.1/dns-over-https/wireformat/
// and https://cloudflare-dns.com/dns-query
// Appears to be supported on 1.1.1.1 ( also supports DNS-TLS)
func (s *DmDns) ForwardHttp(req *dns.Msg) (*dns.Msg, error) {
	id := req.Id
	req.Id = 0

	if len(req.Question) == 0 {
		return nil, errors.New("Empty request")
	}

	m := bufferPoolHttp.Get().([]byte) // make([]byte, int(4096))
	defer bufferPoolHttp.Put(m)
	m = m[0:4096:4096]

	mm, err := req.PackBuffer(m)

	if err != nil {
		return nil, err
	}

	url := "https://" + s.BaseUrl + "/dns?dns=" + base64.RawURLEncoding.EncodeToString(mm)
	hreq, _ := http.NewRequest("GET", url, nil)
	hreq.Header.Add("accept", "application/dns-message")

	ctx, _ := context.WithTimeout(hreq.Context(), 2*time.Second)
	hreq = hreq.WithContext(ctx)
	res, err := s.H2.Do(hreq)
	if err != nil {
		//ClientMetrics.Errors.Add(1)
		//log.Print("DNS Error from http ", err)
		return nil, err
	}
	if res.StatusCode != 200 {
		//ClientMetrics.Errors.Add(1)
		log.Println("DNS Error from http ", s.BaseUrl, req.Question[0].Name, res.StatusCode)
		return nil, errors.New("Error")
	}
	rm, err := ioutil.ReadAll(res.Body)
	res.Body.Close()

	dm := new(dns.Msg)
	dm.Unpack(rm)

	dm.Id = id
	return dm, nil
}

