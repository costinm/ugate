package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"errors"
	"fmt"
	"math/big"
	"time"
)

// Support for ID and authn using certificates.
//
// Private key, certificates and associated helpers.
// An IPv6 address is derived from the public key.
//
type Auth struct {
	// Public part of the Auth info
	//ugate.DMNode
	PublicKey []byte `json:"pub,omitempty"`
	ID        string `json:"id,omitempty"`

	// Primary certificate and private key. Loaded or generated.
	// Currently EC256 - may also support ED in future.
	// This typically includes a [][]byte, crypto.PrivateKey and optionally a parsed leaf
	Cert *tls.Certificate

	// Primary VIP, Created from the PublicKey key, will be included in the self-signed cert.
	VIP6 net.IP

	// Same as VIP6, but as uint64
	VIP64 uint64

	// Blob store - for loading/saving certs and keys. If nil, all is just in memory.
	Config ConfStore

	// Trust domain, should be 'cluster.local' or a real domain with OIDC keys.
	Domain string

	// User name, based on service account or uid.
	Name              string
	Namespace         string
	AllowedNamespaces []string
	TrustedCertPool   *x509.CertPool

	// base64URL encoding of the primary public key.
	// Will be used in JWT header.
	pub64 string

	// Private key to use in both server and client authentication. This is the base of the VIP of the node.
	// ED22519: 32B
	// EC256: DER
	// RSA: DER
	Priv []byte

	//// Secondary private keys.
	//RSACert     *tls.Certificate
	//ED25519Cert *tls.Certificate

	//Cert *x509.Certificate

	Authorized map[string]string
	cfg        *KubeConfig

	// Explicit certificates (lego), key is hostname from file
	//
	CertMap map[string]*tls.Certificate

	// Trusted roots, PEM (each element may have multiple certificates)
	Trusted [][]byte

	// GetCertificateHook allows plugging in an alternative certificate provider. By default files are used.
	GetCertificateHook func(host string) (*tls.Certificate, error)
}

var certValidityPeriod = 100 * 365 * 24 * time.Hour

// NewVapid constructs a new Vapid generator from EC256 public and private keys,
// in base64 uncompressed format.
func NewVapid(publicKey64, privateKey64 string) *Auth {
	publicUncomp, _ := base64.RawURLEncoding.DecodeString(publicKey64)
	privateUncomp, _ := base64.RawURLEncoding.DecodeString(privateKey64)

	x, y := elliptic.Unmarshal(elliptic.P256(), publicUncomp)
	d := new(big.Int).SetBytes(privateUncomp)
	pubkey := ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	pkey := ecdsa.PrivateKey{PublicKey: pubkey, D: d}

	v := &Auth{}
	v.pub64 = publicKey64
	v.PublicKey = publicUncomp
	v.Priv = privateUncomp
	v.PublicKey = elliptic.Marshal(elliptic.P256(), pkey.X, pkey.Y)
	v.Priv = pkey.D.Bytes()

	tlsCert, _, _ := v.generateSelfSigned("ec256", &pkey, "TODO")
	// Required now
	v.Cert = &tlsCert

	return v
}

// Interface for very simple configuration and key loading.
// Can have a simple in-memory, fs implementation, as well
// as K8S, XDS or database backends.
//
// The name is hierachical, in case of K8S or Istio corresponds
// to the type, including namespace.
type ConfStore interface {
	// Get a config blob by name
	Get(name string) ([]byte, error)

	// Save a config blob
	Set(conf string, data []byte) error

	// List the configs starting with a prefix, of a given type
	List(name string, tp string) ([]string, error)
}

func NewAuth(cs ConfStore, name, domain string) *Auth {
	if name == "" {
		if os.Getenv("POD_NAME") != "" {
			name = os.Getenv("POD_NAME") + "." + os.Getenv("POD_NAMESPACE")
		} else {
			name, _ = os.Hostname()
		}
	}
	if domain == "" {
		domain = os.Getenv("DOMAIN")
	}
	if domain == "" {
		domain = "c1.webinf.info"
	}

	auth := &Auth{
		Config:          cs,
		Name:            name,
		Domain:          domain,
		TrustedCertPool: x509.NewCertPool(),
	}

	auth.initCert()

	//if cs != nil {
	//	err := auth.loadCert()
	//	if err != nil {
	//		log.Println("Error loading cert: ", err)
	//	}
	//}
	//
	//// For missing certs - generate new ones.
	//auth.generateCert()
	//auth.tlsCerts = append(auth.tlsCerts, *auth.EC256Cert)

	c0 := auth.Cert
	pk := c0.PrivateKey

	var pubkey crypto.PublicKey
	if priv, ok := pk.(*ecdsa.PrivateKey); ok {
		auth.Priv = priv.D.Bytes()
		auth.PublicKey = elliptic.Marshal(elliptic.P256(), priv.X, priv.Y) // starts with 0x04 == uncompressed curve
		pubkey = priv.Public()
	} else if priv, ok := pk.(*ed25519.PrivateKey); ok {
		auth.Priv = *priv
		edpub := PublicKey(priv)
		auth.PublicKey = edpub.(ed25519.PublicKey)
		pubkey = edpub
	} else if priv, ok := pk.(*rsa.PrivateKey); ok {
		edpub := PublicKey(priv)
		auth.Priv = MarshalPrivateKey(priv)
		auth.PublicKey = MarshalPublicKey(priv.Public())
		pubkey = edpub
	}

	auth.VIP6 = Pub2VIP(auth.PublicKey)
	auth.VIP64 = auth.NodeIDUInt(auth.PublicKey)
	// Based on the primary EC256 key
	auth.pub64 = base64.RawURLEncoding.EncodeToString(auth.PublicKey)
	if auth.ID == "" {
		auth.ID = IDFromPublicKey(pubkey)
	}
	auth.CertMap = auth.GetCerts()

	return auth
}

func (a *Auth) leaf() *x509.Certificate {
	if a.Cert == nil {
		return nil
	}
	if a.Cert.Leaf == nil {
		a.Cert.Leaf, _ = x509.ParseCertificate(a.Cert.Certificate[0])
	}
	return a.Cert.Leaf
}

// Return the DNS-friendly ID, currently base32-encoded SAH256 of EC256 cert.

// From a key pair, generate a tls config with cert.
// Used for Auth and Client servers.
func (auth *Auth) GenerateTLSConfigServer() *tls.Config {
	var crt *tls.Certificate

	crt = auth.Cert

	certs := []tls.Certificate{*crt}

	auth.CertMap["*"] = crt

	return &tls.Config{
		Certificates: certs,
		NextProtos:   []string{"h2"},

		// Will only be called if client supplies SNI and Certificates empty
		GetCertificate: func(ch *tls.ClientHelloInfo) (*tls.Certificate, error) {
			// Log on each new TCP connection, after client hello
			//
			log.Printf("Server/NewConn/CH %s %v %v", ch.ServerName, ch.SupportedProtos, ch.Conn.RemoteAddr())
			// doesn't include :5228
			c, ok := auth.CertMap[ch.ServerName]
			if ok {
				return c, nil
			}
			return crt, nil
		},
	}
}

// Host2ID concerts a Host/:authority or path parameter hostname to a node ID.
//
func (auth *Auth) Host2ID(host string) string {
	col := strings.Index(host, ".")
	if col > 0 {
		host = host[0:col]
	} else {
		col = strings.Index(host, ":")
		if col > 0 {
			host = host[0:col]
		}
	}
	return strings.ToUpper(host)
}

// Get all known certificates from local files. This is used to support
// lego certificates and istio.
//
// "istio" is a special name, set if istio certs are found
//
func (auth *Auth) GetCerts() map[string]*tls.Certificate {
	certMap := map[string]*tls.Certificate{}

	if _, err := os.Stat("./etc/certs/key.pem"); !os.IsNotExist(err) {
		crt, err := tls.LoadX509KeyPair("./etc/certs/cert-chain.pem", "./etc/certs/key.pem")
		if err != nil {
			log.Println("Failed to load system istio certs", err)
		} else {
			certMap["istio"] = &crt
			if crt.Leaf != nil {
				log.Println("Loaded istio cert ", crt.Leaf.URIs)
			}
		}
	}

	legoBase := os.Getenv("HOME") + "/.lego/certificates"
	files, err := ioutil.ReadDir(legoBase)
	if err == nil {
		for _, ff := range files {
			s := ff.Name()
			if strings.HasSuffix(s, ".key") {
				s = s[0 : len(s)-4]
				base := legoBase + "/" + s
				cert, err := tls.LoadX509KeyPair(base+".crt",
					base+".key")
				if err != nil {
					log.Println("ACME: Failed to load ", s, err)
				} else {
					certMap[s] = &cert
					log.Println("ACME: Loaded cert for ", s)
				}
			}
		}
	}

	return certMap
}

func (auth *Auth) Sign(data []byte, sig []byte) {
	hasher := crypto.SHA256.New()
	hasher.Write(data) //[0:64]) // only public key, for debug
	hash := hasher.Sum(nil)

	c0 := auth.Cert
	if ec, ok := c0.PrivateKey.(*ecdsa.PrivateKey); ok {
		r, s, _ := ecdsa.Sign(rand.Reader, ec, hash)
		copy(sig, r.Bytes())
		copy(sig[32:], s.Bytes())
	} else if ed, ok := c0.PrivateKey.(ed25519.PrivateKey); ok {
		sig1, _ := ed.Sign(rand.Reader, hash, nil)
		copy(sig, sig1)
	}
}

func Verify(data []byte, pub []byte, sig []byte) error {
	hasher := crypto.SHA256.New()
	hasher.Write(data) //[0:64]) // only public key, for debug
	hash := hasher.Sum(nil)

	if len(pub) == 64 {
		// Expects 0x4 prefix - we don't send the 4.
		//x, y := elliptic.Unmarshal(curve, pub)
		x := new(big.Int).SetBytes(pub[0:32])
		y := new(big.Int).SetBytes(pub[32:64])
		if !elliptic.P256().IsOnCurve(x, y) {
			return errors.New("invalid public key")
		}

		pubKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
		r := big.NewInt(0).SetBytes(sig[0:32])
		s := big.NewInt(0).SetBytes(sig[32:64])
		match := ecdsa.Verify(pubKey, hash, r, s)
		if match {
			return nil
		} else {
			return errors.New("failed to validate signature ")
		}
	} else if len(pub) == 32 {
		edp := ed25519.PublicKey(pub)
		if ed25519.Verify(edp, hash, sig) {
			return nil
		} else {
			return errors.New("failed to validate signature")
		}
	}
	return errors.New("unknown public key")
}

// Generate a config to be used in a HTTP client, using the primary identity and cert.
func (auth *Auth) GenerateTLSConfigClient() *tls.Config {
	// see transport.go in http onceSetNextProtoDefaults
	return &tls.Config{
		// VerifyPeerCertificate used instead
		InsecureSkipVerify: true,

		Certificates: []tls.Certificate{*auth.Cert},
		// not set on client !! Setting it also disables Auth !
		//NextProtos: nextProtosH2,
	}
}

// WIP: Attempt to get a signed certificate, using Istio protocol.
//func (auth *Auth) GetSignedCert(url string) error {
//	if _, err := os.Stat("./etc/certs/key.pem"); !os.IsNotExist(err) {
//		crt, err := tls.LoadX509KeyPair("./etc/certs/cert-chain.pem", "./etc/certs/key.pem")
//		if err != nil {
//			log.Println("Failed to load system istio certs", err)
//		} else {
//			//auth.RSACert = &crt
//			auth.TlsCerts = append(auth.TlsCerts, crt)
//			if crt.Leaf != nil {
//				log.Println("Loaded istio cert ", crt.Leaf.URIs)
//			}
//		}
//	}
//
//	// Get token
//	// Serialize the proto (raw varint)
//	// Call the grpc ( raw, avoid dep), with token and mTLS
//	// Save it as 'primary' cert.
//
//	return nil
//}

var useED = false

func (auth *Auth) initCert() {
	auth.loadAuthCfg()
	if auth.Cert != nil {
		return // got a cert
	}
	var keyPEM, certPEM []byte
	var tlsCert tls.Certificate
	if useED {
		_, edpk, _ := ed25519.GenerateKey(rand.Reader)
		auth.ID = IDFromPublicKey(PublicKey(edpk))
		tlsCert, keyPEM, certPEM = auth.generateSelfSigned("ed25519", edpk, auth.Name+"."+auth.Domain)
		auth.Cert = &tlsCert
	} else {
		privk, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		auth.ID = IDFromPublicKey(PublicKey(privk))
		tlsCert, keyPEM, certPEM = auth.generateSelfSigned("ec256", privk, auth.Name+"."+auth.Domain)
		auth.Cert = &tlsCert
	}

	kc := &KubeConfig{
		ApiVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "default",
		Contexts: []KubeNamedContext{
			{Name: "default",
				Context: Context{
					Cluster:  "default",
					AuthInfo: auth.ID,
				}},
		},
		Users: []KubeNamedUser{
			{Name: auth.ID,
				User: KubeUser{
					ClientKeyData:         keyPEM,
					ClientCertificateData: certPEM,
				},
			},
		},
		Clusters: []KubeNamedCluster{},
	}

	kcb, _ := json.Marshal(kc)
	if auth.Config != nil {
		auth.Config.Set("kube.json", kcb)
	}
	auth.cfg = kc
}

// loadAuthCfg will attempt to find this node secrets in the store.
// Will try:
// - ./kube.json
// - ./secret/[NAME].[DOMAIN]
func (auth *Auth) loadAuthCfg() {
	if auth.Config == nil {
		return
	}
	rsaKey, _ := auth.Config.Get("key.pem")
	rsaCert, _ := auth.Config.Get("cert-chain.pem")
	// TODO: multiple roots
	rootCert, _ := auth.Config.Get("root-cert.pem")
	if rsaKey != nil && rsaCert != nil {
		tlsCert, err := tls.X509KeyPair(rsaCert, rsaKey)
		if err != nil {
			log.Println("Invalid Istio cert ", err)
		} else {
			auth.Cert = &tlsCert
			if rootCert != nil {
				auth.Trusted = append(auth.Trusted, rootCert)
			}
			for n, c := range tlsCert.Certificate {
				cert, err := x509.ParseCertificate(c)
				if err != nil {
					log.Println("Invalid Istio cert ", err)
					continue
				}
				if n == 0 && len(cert.URIs) > 0 {
					log.Println("ID ", cert.URIs[0], cert.Issuer,
						cert.NotAfter)
					// TODO: get cert fingerprint as well

					//log.Println("Cert: ", cert)
					// TODO: extract domain, ns, name
				} else {
					// org and name are set
					log.Println("Cert: ", cert.Subject.Organization, cert.NotAfter)
				}

			}
			return
		}
	}

	// Single file - more convenient for upload
	// Java supports PKCS12 ( p12, pfx)
	kcfg, _ := auth.Config.Get("kube.json")
	if kcfg == nil {
		kcfg, _ = auth.Config.Get("secret/" + auth.Name + "." + auth.Domain)
	}
	if kcfg == nil {
		return
	}
	kube := &KubeConfig{}
	err := json.Unmarshal(kcfg, kube)
	if err != nil {
		log.Println("Invalid kube config ", err)
		return
	}

	if len(kube.Users) == 0 {
		return
	}

	keyPEM := kube.Users[0].User.ClientKeyData
	certPEM := kube.Users[0].User.ClientCertificateData
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		log.Println("Error loading cert")
		return
	}

	// expect a single user
	// TODO: default context or context env

	auth.cfg = kube
	//auth.TlsCerts = []tls.Certificate{tlsCert}
	auth.Cert = &tlsCert
}

// Load the primary cert - expects a PEM key file
//
// Rejected formats:
// - PKCS12 (p12, pfx) - supported by Java. Too complex.
// - individual files - hard to manage
// -
func (auth *Auth) loadCert() error {

	//keyPEM, _ := auth.Config.Get("ec256-key.pem")
	//certPEM, _ := auth.Config.Get("ec256-cert.pem")
	//if keyPEM != nil && certPEM != nil {
	//	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	//	if err != nil {
	//		return err
	//	}
	//	auth.EC256Cert = &tlsCert
	//}
	//edKey, _ := auth.Config.Get("ed25519-key.pem")
	//edCert, _ := auth.Config.Get("ed25519-cert.pem")
	//if edKey != nil && edCert != nil {
	//	tlsCert, err := tls.X509KeyPair(edCert, edKey)
	//	if err != nil {
	//		return err
	//	}
	//	auth.ED25519Cert = &tlsCert
	//}

	return nil
}

// generateCert will generate the keys and populate the PublicKey/Priv fields.
// Will set privateKey, Priv, PublicKey
// PublicKey, Priv should be saved
func (auth *Auth) generateCert() {
	//var keyPEM []byte
	//var certPEM []byte
	//var tlsCert tls.Certificate
	// The ID is currently based on the EC256 key. Will be included in cert
	//if auth.EC256Cert != nil {
	//	auth.ID = IDFromPublicKey(PublicKey(auth.EC256Cert.PrivateKey))
	//
	//} else {
	//	privk, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	//	auth.ID = IDFromPublicKey(PublicKey(privk))
	//	tlsCert, keyPEM, certPEM = auth.generateSelfSigned("ec256", privk, auth.Name+"."+auth.Domain)
	//	auth.EC256Cert = &tlsCert
	//}

	//if auth.ED25519Cert == nil {
	//	_, edpk, _ := ed25519.GenerateKey(rand.Reader)
	//	tlsCert, _, _ := auth.generateSelfSigned("ed25519", edpk, auth.Name+"."+auth.Domain)
	//	auth.ED25519Cert = &tlsCert
	//}
	//
	//if auth.RSACert == nil {
	//	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	//	tlsCert, _, _ := auth.generateSelfSigned("rsa", priv, auth.Name+"."+auth.Domain)
	//	auth.RSACert = &tlsCert
	//}

}

var (
	MESH_NETWORK = []byte{0xFD, 0x00, 0x00, 0x00, 0x00, 0x00, 0, 0x00}
)

func (auth *Auth) NodeID() []byte {
	return auth.VIP6[8:]
}

// Convert a public key to a VIP. This is the primary ID of the nodes.
// Primary format is the 64-byte EC256 public key.
//
// For RSA, the ASN.1 format of the byte[] is used.
// For ED, the 32-byte raw encoding.
func Pub2VIP(pub []byte) net.IP {
	if pub == nil {
		return nil
	}
	ip6 := make([]byte, 16)
	copy(ip6, MESH_NETWORK)

	binary.BigEndian.PutUint64(ip6[8:], Pub2ID(pub))
	return net.IP(ip6)
}

func (auth *Auth) NodeIDUInt(pub []byte) uint64 {
	return Pub2ID(pub)
}

var enc = base32.StdEncoding.WithPadding(base32.NoPadding)

// IDFromPublicKey returns a node ID based on the
// public key of the node - 52 bytes base32.
func IDFromPublicKey(key crypto.PublicKey) string {
	m := MarshalPublicKey(key)
	if len(m) > 32 {
		sha256 := sha256.New()
		sha256.Write(m)
		m = sha256.Sum([]byte{}) // 302
	}
	return enc.EncodeToString(m)
}

func IDFromPublicKeyBytes(m []byte) string {
	if len(m) > 32 {
		sha256 := sha256.New()
		sha256.Write(m)
		m = sha256.Sum([]byte{}) // 302
	}
	return enc.EncodeToString(m)
}

func IDFromCert(c []*x509.Certificate) string {
	if c == nil || len(c) == 0 {
		return ""
	}
	key := c[0].PublicKey
	m := MarshalPublicKey(key)
	if len(m) > 32 {
		sha256 := sha256.New()
		sha256.Write(m)
		m = sha256.Sum([]byte{}) // 302
	}
	return enc.EncodeToString(m)
}

// Generate a 8-byte identifier from a public key
func Pub2ID(pub []byte) uint64 {
	if len(pub) > 65 {
		sha256 := sha1.New()
		sha256.Write(pub)
		keysha := sha256.Sum([]byte{}) // 302
		return binary.BigEndian.Uint64(keysha[len(keysha)-8:])
	} else {
		// For EC256 and ED - for now just the last bytes
		return binary.BigEndian.Uint64(pub[len(pub)-8:])
	}
}

func RawToCertChain(rawCerts [][]byte) ([]*x509.Certificate, error) {
	chain := make([]*x509.Certificate, len(rawCerts))
	for i := 0; i < len(rawCerts); i++ {
		cert, err := x509.ParseCertificate(rawCerts[i])
		if err != nil {
			return nil, err
		}
		chain[i] = cert
	}
	return chain, nil
}

// PubKeyFromCertChain verifies the certificate chain and extract the remote's public key.
func PubKeyFromCertChain(chain []*x509.Certificate) (crypto.PublicKey, error) {
	if chain == nil || len(chain) == 0 {

	}
	cert := chain[0]

	// Self-signed certificate
	if len(chain) == 1 {
		pool := x509.NewCertPool()
		pool.AddCert(cert)
		if _, err := cert.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
			// If we return an x509 error here, it will be sent on the wire.
			// Wrap the error to avoid that.
			return nil, fmt.Errorf("certificate verification failed: %s", err)
		}
	} else {
		//
		pool := x509.NewCertPool()
		pool.AddCert(chain[len(chain)-1])
		if _, err := cert.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
			// If we return an x509 error here, it will be sent on the wire.
			// Wrap the error to avoid that.
			return nil, fmt.Errorf("chain certificate verification failed: %s", err)
		}
	}

	// IPFS uses a key embedded in a custom extension, and verifies the public key of the cert is signed
	// with the node public key

	// This transport is instead based on standard certs/TLS

	key := cert.PublicKey
	if ec, ok := key.(*ecdsa.PublicKey); ok {
		return ec, nil
	}
	if rsak, ok := key.(*rsa.PublicKey); ok {
		return rsak, nil
	}
	if ed, ok := key.(ed25519.PublicKey); ok {
		return ed, nil
	}

	return nil, errors.New("unknown public key")
}

// Return the self identity. Currently it's using the VIP6 format - may change.
// This is used in Message 'From' and in ReqContext.
func (a *Auth) Self() string {
	return a.VIP6.String()
}

// Check if an identity is authorized for the role.
// The key is in the marshalled format - use PublicKeyBytesRaw to convert a crypto.PublicKey.
//
func (auth *Auth) Auth(key []byte, role string) string {
	roles := auth.Authorized[string(key)]

	return roles
}

func (auth *Auth) genCSR(prefix string, org string, priv crypto.PrivateKey, sans ...string) []byte {
	// Will be based on the JWT
	template := &x509.CertificateRequest{}
	csrBytes, _ := x509.CreateCertificateRequest(rand.Reader, template, priv)

	encodeMsg := "CERTIFICATE REQUEST"
	csrPem := pem.EncodeToMemory(&pem.Block{Type: encodeMsg, Bytes: csrBytes})
	return csrPem
}

func (auth *Auth) SignCSR(csrBytes []byte, org string, sans ...string) ([]byte, error) {
	// Will be based on the JWT

	// Istio uses PEM
	block, _ := pem.Decode(csrBytes)
	if block == nil {
		return nil, fmt.Errorf("certificate signing request is not properly encoded")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse X.509 certificate signing request")
	}

	certDER := auth.signCertDER(csr.PublicKey, auth.Cert.PrivateKey, sans...)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	return certPEM, nil
}

func (auth *Auth) signCertDER(pub crypto.PublicKey, caPrivate crypto.PrivateKey, sans ...string) []byte {
	var notBefore time.Time
	notBefore = time.Now().Add(-1 * time.Hour)

	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, serialNumberLimit)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   sans[0],
			Organization: []string{auth.Domain},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              sans,
		//IPAddresses:           []net.IP{auth.VIP6},
	}
	// IPFS:
	//certKeyPub, err := x509.MarshalPKIXPublicKey(certKey.Public())
	//signature, err := sk.Sign(append([]byte(certificatePrefix), certKeyPub...))
	//value, err := asn1.Marshal(signedKey{
	//	PubKey:    keyBytes,
	//	Signature: signature,
	//})

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, pub, caPrivate)
	if err != nil {
		panic(err)
	}
	return certDER
}

// Generate and save the primary self-signed Certificate
func (auth *Auth) generateSelfSigned(prefix string, priv crypto.PrivateKey, sans ...string) (tls.Certificate, []byte, []byte) {
	return auth.SignCert(priv, priv, sans...)
}

func (auth *Auth) SignCert(priv crypto.PrivateKey, ca crypto.PrivateKey, sans ...string) (tls.Certificate, []byte, []byte) {
	pub := PublicKey(priv)
	certDER := auth.signCertDER(pub, ca, sans...)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	ecb, _ := x509.MarshalPKCS8PrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecb})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		log.Println("Error generating cert ", err)
	}
	return tlsCert, keyPEM, certPEM
}

func MarshalPublicKey(key crypto.PublicKey) []byte {
	if k, ok := key.(ed25519.PublicKey); ok {
		return []byte(k)
	}
	if k, ok := key.(*ecdsa.PublicKey); ok {
		return elliptic.Marshal(elliptic.P256(), k.X, k.Y)
		// starts with 0x04 == uncompressed curve
	}
	if k, ok := key.(*rsa.PublicKey); ok {
		bk := x509.MarshalPKCS1PublicKey(k)
		return bk
	}
	if k, ok := key.([]byte); ok {
		if len(k) == 64 || len(k) == 32 {
			return k
		}
	}

	return nil
}

func MarshalPrivateKey(key crypto.PrivateKey) []byte {
	if k, ok := key.(*rsa.PrivateKey); ok {
		bk := x509.MarshalPKCS1PrivateKey(k)
		return bk
	}

	return nil
}

// Convert a PublicKey to a marshalled format - in the raw format.
// - 32 byte ED25519
// - 65 bytes EC256 ( 0x04 prefix )
// - DER RSA key (PKCS1)
func PublicKeyBytesRaw(key crypto.PublicKey) []byte {
	if ec, ok := key.(*ecdsa.PublicKey); ok {
		// starts with 0x04 == uncompressed curve
		pubbytes := elliptic.Marshal(ec.Curve, ec.X, ec.Y)
		return pubbytes
	}
	if rsak, ok := key.(*rsa.PublicKey); ok {
		pubbytes := x509.MarshalPKCS1PublicKey(rsak)
		return pubbytes
	}
	if ed, ok := key.(ed25519.PublicKey); ok {
		return []byte(ed)
	}
	return nil
}

func PublicKey(key crypto.PrivateKey) crypto.PublicKey {
	if k, ok := key.(ed25519.PrivateKey); ok {
		return k.Public()
	}
	if k, ok := key.(*ecdsa.PrivateKey); ok {
		return k.Public()
	}
	if k, ok := key.(*rsa.PrivateKey); ok {
		return k.Public()
	}

	return nil
}

var (
	oidExtensionSubjectAltName = []int{2, 5, 29, 17}
)

const (
	nameTypeEmail = 1
	nameTypeDNS   = 2
	nameTypeURI   = 6
	nameTypeIP    = 7
)

func getSANExtension(c *x509.Certificate) []byte {
	for _, e := range c.Extensions {
		if e.Id.Equal(oidExtensionSubjectAltName) {
			return e.Value
		}
	}
	return nil
}

// http-related auth
func GetPeerCertBytes(r *http.Request) []byte {
	if r.TLS != nil {
		if len(r.TLS.PeerCertificates) > 0 {
			pke, ok := r.TLS.PeerCertificates[0].PublicKey.(*ecdsa.PublicKey)
			if ok {
				return elliptic.Marshal(elliptic.P256(), pke.X, pke.Y)
			}
			rsap, ok := r.TLS.PeerCertificates[0].PublicKey.(*rsa.PublicKey)
			if ok {
				return x509.MarshalPKCS1PublicKey(rsap)
			}
		}
	}
	return nil
}

func GetResponseCertBytes(r *http.Response) []byte {
	if r.TLS != nil {
		if len(r.TLS.PeerCertificates) > 0 {
			pke, ok := r.TLS.PeerCertificates[0].PublicKey.(*ecdsa.PublicKey)
			if ok {
				return elliptic.Marshal(elliptic.P256(), pke.X, pke.Y)
			}
			rsap, ok := r.TLS.PeerCertificates[0].PublicKey.(*rsa.PublicKey)
			if ok {
				return x509.MarshalPKCS1PublicKey(rsap)
			}
		}
	}
	return nil
}

func GetSAN(c *x509.Certificate) ([]string, error) {
	extension := getSANExtension(c)
	dns := []string{}
	// RFC 5280, 4.2.1.6

	// SubjectAltName ::= GeneralNames
	//
	// GeneralNames ::= SEQUENCE SIZE (1..MAX) OF GeneralName
	//
	// GeneralName ::= CHOICE {
	//      otherName                       [0]     OtherName,
	//      rfc822Name                      [1]     IA5String,
	//      dNSName                         [2]     IA5String,
	//      x400Address                     [3]     ORAddress,
	//      directoryName                   [4]     Name,
	//      ediPartyName                    [5]     EDIPartyName,
	//      uniformResourceIdentifier       [6]     IA5String,
	//      iPAddress                       [7]     OCTET STRING,
	//      registeredID                    [8]     OBJECT IDENTIFIER }
	var seq asn1.RawValue
	rest, err := asn1.Unmarshal(extension, &seq)
	if err != nil {
		return dns, err
	} else if len(rest) != 0 {
		return dns, errors.New("x509: trailing data after X.509 extension")
	}
	if !seq.IsCompound || seq.Tag != 16 || seq.Class != 0 {
		return dns, asn1.StructuralError{Msg: "bad SAN sequence"}
	}

	rest = seq.Bytes
	for len(rest) > 0 {
		var v asn1.RawValue
		rest, err = asn1.Unmarshal(rest, &v)
		if err != nil {
			return dns, err
		}

		if v.Tag == nameTypeDNS {
			dns = append(dns, string(v.Bytes))
		}
	}
	return dns, nil
}

// Get an ID token from platform (GCP, etc)
// curl "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=[AUDIENCE]" \
//  -H "Metadata-Flavor: Google"
//
// May fail and need retry
// TODO: how to detect if running supported ? env ?
func GetMDSIDToken(aud string) string {
	req, _ := http.NewRequest("GET",
		"http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience="+
			aud, nil)
	req.Header.Add("Metadata-Flavor", "Google")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	rb, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return ""
	}
	return string(rb)
}
