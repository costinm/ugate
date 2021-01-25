package ugate

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"testing"

	"github.com/costinm/ugate/pkg/pipe"
	"golang.org/x/crypto/ed25519"
)

func initServer(basePort int, cfg *GateCfg) *UGate {
	if cfg == nil {
		cfg = &GateCfg{
			BasePort: basePort,
			//H2R: map[string]string{
			//	"127.0.0.1:15007": "",
			//},
		}
	}
	ug := NewGate(&net.Dialer{}, nil, cfg)

	log.Println("Starting server EC", IDFromPublicKey(PublicKey(ug.Auth.EC256Cert.PrivateKey)))
	if ug.Auth.ED25519Cert != nil {
		log.Println("Starting server ED", IDFromPublicKey(PublicKey(ug.Auth.ED25519Cert.PrivateKey)))
	}
	if ug.Auth.RSACert != nil {
		log.Println("Starting server RSA", IDFromPublicKey(PublicKey(ug.Auth.RSACert.PrivateKey)))
	}

	// Echo - TCP
	_, _, _ = ug.Add(&Listener{
		Address:   fmt.Sprintf("0.0.0.0:%d", basePort+11),
		Protocol:  "tls",
		Handler:   &EchoHandler{},
	})
	_, _, _ = ug.Add(&Listener{
		Address: fmt.Sprintf("0.0.0.0:%d", basePort+12),
		Handler: &EchoHandler{},
	})
	ug.Mux.Handle("/", &EchoHandler{})
	return ug
}

var chunk1 = []byte("Hello world")
var chunk2 = []byte("chunk2")

func checkEcho(in io.Reader, out io.Writer) (string, error) {
	d := make([]byte, 2048)
	// Start with a write - client send first (echo will wait, to verify that body is not cached)
	_, err := out.Write(chunk1)
	if err != nil {
		return "", err
	}

	//ab.SetDeadline(time.Now().Add(5 * time.Second))
	n, err := in.Read(d)
	if err != nil {
		return "", err
	}

	idx := bytes.IndexByte(d[0:n], '\n')
	if idx < 0 {
		return string(d[0:n]), errors.New("missing header")
	}
	js := string(d[0:idx])

	// Server writes 2 chunkes - the header and what we wrote
	n, err = in.Read(d)
	if err != nil {
		return "", err
	}

	if !bytes.Equal(chunk1, d[0:n]) {
		return js, errors.New("miss-matched result " + string(d[0:n]))
	}

	_, err = out.Write(chunk2)
	if err != nil {
		return "", err
	}

	n, err = in.Read(d)
	if err != nil {
		return "", err
	}

	if !bytes.Equal(chunk2, d[0:n]) {
		return js, errors.New("miss-matched result " + string(d[0:n]))
	}

	/*	_, err = out.Write([]byte("close\n"))
		if err != nil {
			return "", err
		}
	*/
	if cw, ok := out.(CloseWriter); ok {
		cw.CloseWrite()
	} else {
		out.(io.Closer).Close()
	}
	n, err = in.Read(d)
	if err != io.EOF {
		log.Println("unexpected ", err)
	}

	return js, nil
}

func xTestLive(t *testing.T) {
	m := "10.1.10.228:15007"
	ag := initServer(6300, &GateCfg{
		BasePort: 6300,
		H2R: map[string]string{
			//"h.webinf.info:15007": "",
			m: "",
		},
	})
	//ag.h2Handler.UpdateReverseAccept()

	//err := ag.h2Handler.maintainRemoteAccept("h.webinf.info:15007", "")
	//if err != nil {
	//	t.Fatal("Failed to RA", err)
	//}

	//p := pipe.New()
	//r, _ := http.NewRequest("POST",
	//	"https://" + m + "/dm/" + IDFromPublicKey(ag.Auth.EC256PrivateKey.Public()), p)
	//res, err := ag.h2Handler.RoundTrip(r)
	//
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//res1, err := checkEcho(res.Body, p)
	//if err != nil {
	//	t.Fatal(err)
	//}

	log.Println(ag)
	//log.Println(res1, ag)
	select {}
}

func TestSrv(t *testing.T) {
	ag := initServer(6000, nil)
	bg := initServer(6100, nil)
	cg := initServer(6200, nil)

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
		r, _ := http.NewRequest("POST", "https://127.0.0.1:6107/dm/"+"127.0.0.1:6112", p)
		res, err := ag.h2Handler.RoundTrip(r)
		if err != nil {
			t.Fatal(err)
		}

		res1, err := checkEcho(res.Body, p)
		if err != nil {
			t.Fatal(err)
		}

		log.Println(res1, ag, bg)
	})

	t.Run("H2R", func(t *testing.T) {
		ag.Config.H2R = map[string]string{
			"127.0.0.1:6107": "",
		}
		ag.h2Handler.UpdateReverseAccept()
		// Connecting to Bob's gateway (from c). Request should go to Alice.
		//
		p := pipe.New()
		r, _ := http.NewRequest("POST",
			"https://127.0.0.1:6107/dm/"+ag.Auth.ID, p)
		res, err := cg.h2Handler.RoundTrip(r)

		if err != nil {
			t.Fatal(err)
		}

		res1, err := checkEcho(res.Body, p)
		if err != nil {
			t.Fatal(err)
		}

		log.Println(res1, ag, bg)

		ag.Config.H2R = map[string]string{
		}
		ag.h2Handler.UpdateReverseAccept()
		if len(ag.h2Handler.reverse) > 0 {
			t.Error("Failed to disconnect")
		}
	})
}

// Run a suite of tests with a specific key, to repeat the tests for all types of keys.
func testKey(k crypto.PrivateKey, t *testing.T) {
	pk := PublicKey(k)
	if pk == nil {
		t.Fatal("Invalid public")
	}
	id := IDFromPublicKey(pk)
	//crt, err := KeyToCertificate(k)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//chain, err := RawToCertChain(crt.Certificate)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//certPub, err := PubKeyFromCertChain(chain)
	//if id != IDFromPublicKey(certPub) {
	//	t.Fatal("Cert chain public key not matching", id, IDFromPublicKey(certPub))
	//}

	t.Log("Key ID:", id)
}

type MemCfg struct {
	data map[string][]byte
}

func (m MemCfg) Get(name string) ([]byte, error) {
	return m.data[name], nil
}

func (m MemCfg) Set(conf string, data []byte) error {
	m.data[conf] = data
	return nil
}

func (m MemCfg) List(name string, tp string) ([]string, error) {
	return []string{}, nil
}

func TestCrypto(t *testing.T) {
	t.Run("AuthInit", func(t *testing.T) {
		cfg := &MemCfg{data: map[string][]byte{}}
		a := NewAuth(cfg, "", "")

		if a.RSACert == nil {
			t.Error("Missing RSA")
		}
		if a.EC256Cert == nil {
			t.Error("Missing EC")
		}
		if a.ED25519Cert == nil {
			t.Error("Missing ED")
		}

		b := NewAuth(cfg, "", "")
		if !bytes.Equal(a.ED25519Cert.PrivateKey.(ed25519.PrivateKey),
			b.ED25519Cert.PrivateKey.(ed25519.PrivateKey)) {
			t.Error("Error loading")
		}
		if !a.RSACert.PrivateKey.(*rsa.PrivateKey).Equal(
			b.RSACert.PrivateKey) {
			t.Error("Error loading")
		}
		if !a.EC256Cert.PrivateKey.(*ecdsa.PrivateKey).Equal(b.EC256Cert.PrivateKey) {
			t.Error("Error loading")
		}
		testKey(a.ED25519Cert.PrivateKey, t)
		testKey(a.RSACert.PrivateKey, t)
		testKey(a.EC256Cert, t)
	})
}

func TestUGate(t *testing.T) {
	td := &net.Dialer{}
	ug := NewGate(td, nil, nil)

	basePort := 2900
	ug.Add(&Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", basePort+100),
		Protocol: "echo",
	})
	ug.Add(&Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", basePort+101),
		Protocol: "static",
	})
	ug.Add(&Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", basePort+102),
		Protocol: "delay",
	})
	ug.Add(&Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", basePort+106),
		Protocol: "tls",
	})
	ug.Add(&Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", basePort+103),
		Protocol: "socks5",
	})
	ug.Add(&Listener{
		Address:   fmt.Sprintf("0.0.0.0:%d", basePort+105),
		ForwardTo: "localhost:3000",
	})

	ug.Add(&Listener{
		Address: fmt.Sprintf("0.0.0.0:%d", basePort+106),
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

func xTestKube(t *testing.T) {
	d, _ := ioutil.ReadFile("testdata/kube.json")
	kube := &KubeConfig{}
	err := json.Unmarshal(d, kube)
	if err != nil {
		t.Fatal("Invalid kube config ", err)
	}
	//log.Printf("%#v\n", kube)
	var pubk crypto.PublicKey
	for _, c := range kube.Clusters {
		//log.Println(string(c.Cluster.CertificateAuthorityData))
		block, _ := pem.Decode(c.Cluster.CertificateAuthorityData)
		xc, _ := x509.ParseCertificate(block.Bytes)
		log.Printf("%#v\n", xc.Subject)
		pubk = xc.PublicKey
	}

	scrt, _ := ioutil.ReadFile("testdata/server.crt")
	block, _ := pem.Decode(scrt)
	xc, _ := x509.ParseCertificate(block.Bytes)
	log.Printf("%#v\n", xc.Subject)
	pubk1 := xc.PublicKey

	for _, a := range kube.Users {
		if a.User.Token != "" {
			h, t, txt, sig, _ := jwtRawParse(a.User.Token)
			log.Printf("%#v %#v\n", h, t)

			if h.Alg == "RS256" {
				rsak := pubk.(*rsa.PublicKey)
				hasher := crypto.SHA256.New()
				hasher.Write(txt)
				hashed := hasher.Sum(nil)
				err = rsa.VerifyPKCS1v15(rsak,crypto.SHA256, hashed, sig)
				if err != nil {
					log.Println("K8S Root Certificate not a signer")
				}
				err = rsa.VerifyPKCS1v15(pubk1.(*rsa.PublicKey),crypto.SHA256, hashed, sig)
				if err != nil {
					log.Println("K8S Server Certificate not a signer")
				}
			}

		}

		if a.User.ClientKeyData != nil {

		}
		if a.User.ClientCertificateData != nil {

		}
	}
}
