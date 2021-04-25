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
	"net/http"
	"testing"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/auth"
	"github.com/costinm/ugate/pkg/cfgfs"
	"github.com/costinm/ugate/pkg/pipe"
	"github.com/costinm/ugate/test"
)


func TestSrv(t *testing.T) {
	// xx07 -> BTS port
	// xx12 -> plain text echo
	// "/dm/" -> POST-based tunnel ( more portable than CONNECT )

	// Bob connected to Carol
	bob := test.InitTestServer(test.BOB_KEYS, &ugate.GateCfg{
		BasePort: 6100,
		Name:     "bob",
	}, nil)

	// Alice connected to Bob
	alice := test.InitTestServer(test.ALICE_KEYS, &ugate.GateCfg{
		BasePort: 6000,
		Name:     "alice",
		H2R: map[string]string{
			"bob": "-",
		},
		Hosts: map[string]*ugate.DMNode{
			"bob": &ugate.DMNode{
				Addr: "127.0.0.1:6107",
			},
		},
	}, nil)


	//carol := test.InitTestServer(test.CAROL_KEYS, &ugate.GateCfg{
	//	BasePort: 6200,
	//	Name:     "carol",
	//}, nil)

	//http2.DebugGoroutines = true

	t.Run("Echo-tcp", func(t *testing.T) {
		ab, err := alice.DialContext(context.Background(), "tcp",
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
		ab, err := alice.DialContext(context.Background(), "tls",
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
		r, _ := http.NewRequest("POST", "https://127.0.0.1:6107/dm/"+"127.0.0.1:6112", p)
		res, err := alice.RoundTrip(r)
		if err != nil {
			t.Fatal(err)
		}

		res1, err := test.CheckEcho(res.Body, p)
		if err != nil {
			t.Fatal(err)
		}

		log.Println(res1, alice, bob)
	})

	// TODO: perm checks for egress
	// TODO: same test for ingress ( localhost:port ) and internal listeners

	//// Bob -> H2R -> Alice
	//t.Run("H2R1", func(t *testing.T) {
	//	p := pipe.New()
	//	r, _ := http.NewRequest("POST", "https://" + alice.Auth.ID + "/dm/127.0.0.1:6112", p)
	//	res, err := bob.RoundTrip(r)
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//	nc := ugate.NewStreamRequestOut(r, p, res, nil)
	//
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//	res1, err := test.CheckEcho(nc, nc)
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//
	//	log.Println(res1, alice, bob)
	//})
	//
	//// Alice -- reverse tunnel --> Bob
	//// Carol --> Bob -- via H2R --> Alice
	//t.Run("H2R", func(t *testing.T) {
	//	alice.H2Handler.UpdateReverseAccept()
	//	// Connecting to Bob's gateway (from c). Request should go to Alice.
	//	//
	//	p := pipe.New()
	//	r, _ := http.NewRequest("POST", "https://127.0.0.1:6107/dm/"+alice.Auth.ID + "/dm/127.0.0.1:6112", p)
	//	res, err := carol.RoundTrip(r)
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//	nc := ugate.NewStreamRequestOut(r, p, res, nil)
	//
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//	res1, err := test.CheckEcho(nc, nc)
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//
	//	log.Println(res1, alice, bob)
	//})
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

func TestCrypto(t *testing.T) {
	t.Run("AuthInit", func(t *testing.T) {
		cfg := cfgfs.NewConf()
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

func BenchmarkUGate(t *testing.B) {
	// WIP
}

var tlsConfigInsecure = &tls.Config{InsecureSkipVerify: true}

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
