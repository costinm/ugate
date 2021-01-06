package ugate

import (
	"crypto"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"context"

	"github.com/costinm/ugate/pkg/pipe"
)

// Run a suite of tests with a specific key, to repeat the tests for all types of keys.
func testKey(k crypto.PrivateKey, t *testing.T) {
	pk := PublicKey(k)
	if pk == nil {
		t.Fatal("Invalid public")
	}
	id := IDFromPublicKey(pk)

	kb, err := MarshalPrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	_, err = UnmarshalPrivateKey(kb)
	if err != nil {
		t.Fatal(err)
	}


	crt, err := KeyToCertificate(k)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := RawToCertChain(crt.Certificate)
	if err != nil {
		t.Fatal(err)
	}
	certPub, err := PubKeyFromCertChain(chain)
	if id != IDFromPublicKey(certPub) {
		t.Fatal("Cert chain public key not matching", id, IDFromPublicKey(certPub))
	}

	t.Log("Key ID:", id)

	ug := NewGate(&net.Dialer{}, nil)
	_, _, _ = ug.Add(&ListenerConf{

	})

}

func initServer(basePort int) *UGate {
	ug := NewGate(&net.Dialer{}, nil)
	ug.DefaultPorts(basePort)

	log.Println("Starting server EC", IDFromPublicKey(ug.Auth.EC256PrivateKey.Public()))
	if ug.Auth.EDPrivate != nil {
		log.Println("Starting server ED", IDFromPublicKey(ug.Auth.EDPrivate.Public()))
	}
	if ug.Auth.RSAPrivate != nil {
		log.Println("Starting server RSA", IDFromPublicKey(ug.Auth.RSAPrivate.Public()))
	}


	// Echo - TCP
	_, _, _ = ug.Add(&ListenerConf{
		Port: basePort + 11,
		Protocol: "tls",
		TLSConfig: ug.TLSConfig,
		Handler:  &EchoHandler{},
	})
	_, _, _ = ug.Add(&ListenerConf{
		Port: basePort + 12,
		Handler:  &EchoHandler{},
	})
	ug.h2Handler.Mux.Handle("/", &EchoHandler{})
	return ug
}

func checkEcho(in io.Reader, out io.Writer) (string, error){
	d := make([]byte, 2048)
	_, err := out.Write([]byte("Hello world"))
	if err != nil {
		return "", err
	}

	//ab.SetDeadline(time.Now().Add(5 * time.Second))
	n, err := in.Read(d)
	if err != nil {
		return "", err
	}
	// Expect a JSON with request info, plus what we wrote.

	return string(d[0:n]), nil
}

func TestSrv(t *testing.T) {
	ag := initServer( 6000)
	bg := initServer( 6100)
	cg := initServer( 6200)


	//http2.DebugGoroutines = true
	t.Run("Echo-tcp", func(t *testing.T) {
		// Can also use net.Dial
		ab, err := ag.DialContext(context.Background(), "tcp",
			fmt.Sprintf("127.0.0.1:%d", 6112))
		if err != nil {
			t.Fatal(err)
		}
		res, err := checkEcho(ab, ab)
		if err != nil {
			t.Fatal(err)
		}
		log.Println("Result ", res)
	})

	// TLS Echo server, on 6111
	t.Run("Echo-tls", func(t *testing.T) {
		ab, err := ag.DialContext(context.Background(), "tls",
			fmt.Sprintf("127.0.0.1:%d", 6111))
		if err != nil {
			t.Fatal(err)
		}
		res, err := checkEcho(ab, ab)
		if err != nil {
			t.Fatal(err)
		}
		mc := ab.(MetaConn)
		meta := mc.Meta()
		log.Println("Result ", res, meta)
	})

	t.Run("H2-egress", func(t *testing.T) {
		// This is a H2 request that is forwarded to a stream.
		p := pipe.New()
		r, _ := http.NewRequest("POST", "https://127.0.0.1:6107/dm/" + "127.0.0.1:6112", p)
		res, err := ag.h2Handler.RoundTrip(r)
		if err != nil {
			t.Fatal(err)
		}

		d := make([]byte, 1024)

		_, err = p.Write([]byte{1})
		if err != nil {
			t.Fatal(err)
		}

		//ab.SetDeadline(time.Now().Add(5 * time.Second))
		n, err := res.Body.Read(d)
		if err != nil {
			t.Fatal(err)
		}

		log.Println(string(d[0:n]), ag, bg)
	})

	t.Run("H2R", func(t *testing.T) {
		go func() {
			// Bob's gateway acts as listener/accepter for Alice.
			// Alice maintains the reverse H2 connection open.
			err := ag.h2Handler.ReverseAccept("127.0.0.1:6107")
			log.Println("reverse accept closed", err)
		}()

		time.Sleep(1 * time.Second)
		// Connecting to Bob's gateway (from c). Request should go to Alice.
		//
		p := pipe.New()
		r, _ := http.NewRequest("POST",
			"https://127.0.0.1:6107/dm/" + IDFromPublicKey(ag.Auth.EC256PrivateKey.Public()), p)
		res, err := cg.h2Handler.RoundTrip(r)

		if err != nil {
			t.Fatal(err)
		}

		d := make([]byte, 1024)

		_, err = p.Write([]byte{1})
		if err != nil {
			t.Fatal(err)
		}

		//ab.SetDeadline(time.Now().Add(5 * time.Second))
		n, err := res.Body.Read(d)
		if err != nil {
			t.Fatal(err)
		}

		log.Println(string(d[0:n]), ag, bg)
	})

}

func TestCrypto(t *testing.T) {
	t.Run("ED25519", func(t *testing.T) {
		ed := GenerateKeyPair()
		testKey(ed, t)

		al := GenerateKeyPair()
		testKey(al, t)


	})
	t.Run("RSA", func(t *testing.T) {
		ed := GenerateRSAKeyPair()
		testKey(ed, t)
	})
	t.Run("EC256", func(t *testing.T) {
		ed := GenerateEC256KeyPair()
		testKey(ed, t)
	})

}


func TestUGate(t *testing.T) {
	td := &net.Dialer{}
	ug := NewGate(td, nil)

	ug.Add(&ListenerConf{
		Port: 3000,
		Protocol: "echo",
	})
	ug.Add(&ListenerConf{
		Port: 3001,
		Protocol: "static",
	})
	ug.Add(&ListenerConf{
		Port: 3002,
		Protocol: "delay",
	})
	ug.Add(&ListenerConf{
		Port: 3006,
		Protocol: "tls",
	})
	ug.Add(&ListenerConf{
		Port:     3003,
		Host:     "127.0.0.1:3003",
		Protocol: "socks5",
	})
	// In-process dialer (ssh, etc)
	ug.Add(&ListenerConf{
		Port:   3004,
		Dialer: td,
	})
	ug.Add(&ListenerConf{
		Port: 3005,
		Remote: "localhost:3000",
	})

	ug.Add(&ListenerConf{
		Port: 3006,
		Handler: &EchoHandler{},
	})

}

func BenchmarkUGate(t *testing.B) {
// WIP
}

var tlsConfigInsecure = &tls.Config{InsecureSkipVerify: true}

//func Test_Service(t *testing.T) {
//	mux := http.NewServeMux()
//
//	// Real TLS server listener, httptest.Server
//	srv := httptest.NewUnstartedServer(mux)
//	srv.EnableHTTP2 = true
//	srv.StartTLS()
//	defer srv.Close()
//
//	http2.ConfigureServer(srv.Config, &http2.Server{})
//
//	log.Println(srv.URL)
//	url := srv.URL
//
//	http2.VerboseLogs = true
//
//	tr := &http.Transport{
//		TLSClientConfig: tlsConfigInsecure,
//	}
//	hc := http.Client{
//		Transport: tr,
//	}
//
//	res, err := hc.Get(url + "/push/mon/1234")
//	if err != nil {
//		t.Fatal("subscribe", err)
//	}
//	loc := res.Header.Get("location")
//	if len(loc) == 0 {
//		t.Fatal("location", res)
//	}
//}
