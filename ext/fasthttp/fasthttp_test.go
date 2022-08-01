package fasthttp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"os/signal"
	"sync"
	"testing"
	"time"

	"github.com/buaazp/fasthttprouter"
	"github.com/dgrr/fastws"
	"github.com/dgrr/http2"
	"github.com/valyala/fasthttp"
)

func TestFastHTTP2(t *testing.T) {
	c1, c2 := net.Pipe()
	cc := http2.NewConn(c1, http2.ConnOpts{
		PingInterval:        30 * time.Second,
		DisablePingChecking: false,
		OnDisconnect: func(c *http2.Conn) {
			log.Println("Client closed")
		},
	})

	_ = http2.NewConn(c2, http2.ConnOpts{
		PingInterval:        30 * time.Second,
		DisablePingChecking: false,
		OnDisconnect: func(c *http2.Conn) {
			log.Println("Client closed")
		},
	})

	err := cc.Handshake()
	if err != nil {
		t.Fatal(err, cc.LastErr())
	}

	if !cc.CanOpenStream() {
		t.Fatal("Can't open stream")
	}

	if !cc.Closed() {
		t.Fatal("closed")
	}

	ch := make(chan error, 1)
	rctx := &http2.Ctx{
		Request:  nil,
		Response: nil,
		Err:      nil,
	}

	// Queue the request on conn. No streaming.
	cc.Write(rctx)

	select {
	case err = <-ch:
	}

}

func TestFastHTTP(t *testing.T) {

	router := fasthttprouter.New()
	router.GET("/", rootHandler)
	router.GET("/ws", fastws.Upgrade(wsHandler))

	server := fasthttp.Server{
		Handler: router.Handler,
	}
	go func() {
		if err := server.ListenAndServe(":8081"); err != nil {
			log.Fatalln(err)
		}
	}()

	fmt.Println("Visit http://localhost:8081")

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
	signal.Stop(sigCh)
	signal.Reset(os.Interrupt)
	server.Shutdown()
}

func wsHandler(conn *fastws.Conn) {
	fmt.Printf("Opened connection\n")

	conn.WriteString("Hello")

	var msg []byte
	var wg sync.WaitGroup
	mch := make(chan string, 128)

	wg.Add(2)
	go func() {
		defer wg.Done()
		defer close(mch)

		var err error
		for {
			_, msg, err = conn.ReadMessage(msg[:0])
			if err != nil {
				if err != fastws.EOF {
					fmt.Fprintf(os.Stderr, "error reading message: %s\n", err)
				}
				break
			}
			mch <- string(msg)
		}
	}()

	go func() {
		defer wg.Done()
		for m := range mch {
			//bf := bytebufferpool.Get()
			//		bf.B = v.MarshalTo(bf.B[:0])
			//
			//		fr := websocket.AcquireFrame()
			//		fr.SetPayload(bf.B)
			//
			//		if !bytes.Equal(fr.Payload(), []byte(`{"a":20,"b":"hello world","c":{"z":"inner object"}}`)) {
			//			b.Fatal("unequal")
			//		}
			//
			//		bytebufferpool.Put(bf)
			//		websocket.ReleaseFrame(fr)
			_, err := conn.WriteString(m)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error writing message: %s\n", err)
				break
			}
			time.Sleep(time.Millisecond * 100)
		}
	}()

	wg.Wait()

	fmt.Printf("Closed connection\n")
}

func rootHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("text/html")
	fmt.Fprintln(ctx, `<!DOCTYPE html>
<html>
  <head>
    <meta charset="UTF-8"/>
    <title>Sample of websocket with Golang</title>
  </head>
  <body>
		<div id="text"></div>
    <script>
      var ws = new WebSocket("ws://localhost:8081/ws");
      ws.onmessage = function(e) {
				var d = document.createElement("div");
        d.innerHTML = e.data;
				ws.send(e.data);
        document.getElementById("text").appendChild(d);
      }
			ws.onclose = function(e){
				var d = document.createElement("div");
				d.innerHTML = "CLOSED";
        document.getElementById("text").appendChild(d);
			}
    </script>
  </body>
</html>`)
}

func TestHTTP2(t *testing.T) {
	cert, priv, err := GenerateTestCertificate("localhost:8443")
	if err != nil {
		log.Fatalln(err)
	}

	s := &fasthttp.Server{
		Handler: requestHandler,
		Name:    "http2 test",
	}
	err = s.AppendCertEmbed(cert, priv)
	if err != nil {
		log.Fatalln(err)
	}

	http2.ConfigureServer(s, http2.ServerConfig{})

	err = s.ListenAndServeTLS(":8443", "", "")
	if err != nil {
		log.Fatalln(err)
	}
}

func requestHandler(ctx *fasthttp.RequestCtx) {
	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()

	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	ctx.Request.CopyTo(req)

	req.Header.SetProtocol("HTTP/1.1")
	req.SetRequestURI("http://localhost:8080" + string(ctx.RequestURI()))

	if err := fasthttp.Do(req, res); err != nil {
		ctx.Error("gateway error", fasthttp.StatusBadGateway)
		return
	}

	res.CopyTo(&ctx.Response)
}

// GenerateTestCertificate generates a test certificate and private key based on the given host.
func GenerateTestCertificate(host string) ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"fasthttp test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		DNSNames:              []string{host},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certBytes, err := x509.CreateCertificate(
		rand.Reader, cert, cert, &priv.PublicKey, priv,
	)

	p := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(priv),
		},
	)

	b := pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certBytes,
		},
	)

	return b, p, err
}
