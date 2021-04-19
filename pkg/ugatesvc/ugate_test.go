package ugatesvc_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"testing"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/auth"
	"github.com/costinm/ugate/pkg/pipe"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/costinm/ugate/test"
)



func xTestLive(t *testing.T) {
	m := "10.1.10.228:15007"
	ag := test.InitTestServer("", &ugate.GateCfg{
		BasePort: 6300,
		H2R: map[string]string{
			//"h.webinf.info:15007": "",
			m: "",
		},
	})
	//ag.h2Handler.UpdateReverseAccept()

	//err := ag.h2Handler.maintainPinnedConnection("h.webinf.info:15007", "")
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
	//res1, err := CheckEcho(res.Body, p)
	//if err != nil {
	//	t.Fatal(err)
	//}

	log.Println(ag)
	//log.Println(res1, ag)
	select {}
}

func TestSrv(t *testing.T) {
	// Bob accepts POST and H2R tunnels
	// Bob connected to Carol
	bg := test.InitTestServer(test.BOB_KEYS, &ugate.GateCfg{
		BasePort: 6100,
		Name: "bob",
	})

	// Alice connected to Bob
	ag := test.InitTestServer(test.ALICE_KEYS, &ugate.GateCfg{
		BasePort: 6000,
		Name: "alice",
		H2R: map[string]string{
			"bob": "-",
		},
		Hosts: map[string] *ugate.DMNode {
			"bob": &ugate.DMNode{
				Addr: "127.0.0.1:6107",
			},
		},
	})


	// Carol accepts only POST tunnels
	cg := test.InitTestServer(test.CAROL_KEYS, &ugate.GateCfg{
		BasePort: 6200,
		Name: "carol",
	})

	//http2.DebugGoroutines = true

	t.Run("Echo-tcp", func(t *testing.T) {
		ab, err := ag.DialContext(context.Background(), "tcp",
			fmt.Sprintf("127.0.0.1:%d", 6112))
		if err != nil {
			t.Fatal(err)
		}
		res, err := test.CheckEcho(ab, ab)
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
		// TODO: verify the identity, cert, etc
		res, err := test.CheckEcho(ab, ab)
		if err != nil {
			t.Fatal(err)
		}
		mc := ab.(ugate.MetaConn)
		meta := mc.Meta()
		log.Println("Result ", res, meta)
	})

	// This is a H2 (BTS) request that is forwarded to a TCP stream handler.
	// Alice -BTS-> Bob -TCP-> Carol
	t.Run("H2-egress", func(t *testing.T) {
		p := pipe.New()
		// xx07 -> BTS port
		// xx12 -> plain text echo
		// "/dm/" -> POST-based tunnel ( more portable than CONNECT )
		r, _ := http.NewRequest("POST", "https://127.0.0.1:6107/dm/"+"127.0.0.1:6112", p)
		res, err := ag.RoundTrip(r)
		if err != nil {
			t.Fatal(err)
		}

		res1, err := test.CheckEcho(res.Body, p)
		if err != nil {
			t.Fatal(err)
		}

		log.Println(res1, ag, bg)
	})

	// TODO: perm checks for egress
	// TODO: same test for ingress ( localhost:port ) and internal listeners

	// Alice -- reverse tunnel --> Bob
	// Carol --> Bob -- via H2R --> Alice
	t.Run("H2R", func(t *testing.T) {
		ag.H2Handler.UpdateReverseAccept()
		// Connecting to Bob's gateway (from c). Request should go to Alice.
		//
		nc, err := cg.DialContext(context.Background(), "url",
			"https://127.0.0.1:6107/dm/"+ag.Auth.ID + "/dm/127.0.0.1:6112")
		if err != nil {
			t.Fatal(err)
		}
		res1, err := test.CheckEcho(nc, nc)
		if err != nil {
			t.Fatal(err)
		}

		log.Println(res1, ag, bg)

		//ag.Config.H2R = map[string]string{
		//}
		//ag.H2Handler.UpdateReverseAccept()
		if len(ag.H2Handler.Reverse) > 0 {
			t.Error("Failed to disconnect")
		}
	})
}

// Run a suite of tests with a specific key, to repeat the tests for all types of keys.
func testKey(k crypto.PrivateKey, t *testing.T) {
	pk := auth.PublicKey(k)
	if pk == nil {
		t.Fatal("Invalid public")
	}
	id := auth.IDFromPublicKey(pk)
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
		a := auth.NewAuth(cfg, "", "")

		//if a.RSACert == nil {
		//	t.Error("Missing RSA")
		//}
		if a.Cert == nil {
			t.Error("Missing EC")
		}
		//if a.ED25519Cert == nil {
		//	t.Error("Missing ED")
		//}

		b := auth.NewAuth(cfg, "", "")
		//if !bytes.Equal(a.ED25519Cert.PrivateKey.(ed25519.PrivateKey),
		//	b.ED25519Cert.PrivateKey.(ed25519.PrivateKey)) {
		//	t.Error("Error loading")
		//}
		//if !a.RSACert.PrivateKey.(*rsa.PrivateKey).Equal(
		//	b.RSACert.PrivateKey) {
		//	t.Error("Error loading")
		//}
		if !a.Cert.PrivateKey.(*ecdsa.PrivateKey).Equal(b.Cert.PrivateKey) {
			t.Error("Error loading")
		}
		//testKey(a.ED25519Cert.PrivateKey, t)
		//testKey(a.RSACert.PrivateKey, t)
		testKey(a.Cert, t)
	})
}

func TestUGate(t *testing.T) {
	td := &net.Dialer{}
	ug := ugatesvc.NewGate(td, nil, nil, nil)

	basePort := 2900
	ug.Add(&ugate.Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", basePort+100),
		Protocol: "echo",
	})
	ug.Add(&ugate.Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", basePort+101),
		Protocol: "static",
	})
	ug.Add(&ugate.Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", basePort+102),
		Protocol: "delay",
	})
	ug.Add(&ugate.Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", basePort+106),
		Protocol: "tls",
	})
	ug.Add(&ugate.Listener{
		Address:  fmt.Sprintf("0.0.0.0:%d", basePort+103),
		Protocol: "socks5",
	})
	ug.Add(&ugate.Listener{
		Address:   fmt.Sprintf("0.0.0.0:%d", basePort+105),
		ForwardTo: "localhost:3000",
	})

	ug.Add(&ugate.Listener{
		Address: fmt.Sprintf("0.0.0.0:%d", basePort+106),
		Handler: &ugatesvc.EchoHandler{},
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
	kube := &auth.KubeConfig{}
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
			h, t, txt, sig, _ := auth.JwtRawParse(a.User.Token)
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
