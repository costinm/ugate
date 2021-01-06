package ugate

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"

	"errors"
	"fmt"
	"math/big"
	"time"
)


// Support for ID and authn using certificates.
// Private key, certificates and associated helpers.
// An IPv6 address is derived from the public key.
//
type Auth struct {
	// Blob store - for loading/saving certs and keys.
	Config ConfStore

	// Trust domain
	Domain string

	// May be sa.namespace form
	Name string

	// Primary public key of the node. Using EC256 key for webpush as primary.
	// EC256: 65 bytes, uncompressed format
	// RSA: DER
	// ED25519: 32B
	Pub []byte

	// Private key to use in both server and client authentication. This is the base of the VIP of the node.
	// ED22519: 32B
	// EC256: DER
	// RSA: DER
	Priv []byte

	// base64URL encoding of the primary public key.
	// Will be added to Crypto-Keys p256ecdsa header field.
	PubKey string

	// Certificates associated with this node.
	tlsCerts []tls.Certificate

	// Primary VIP, Created from the Pub key, will be included in the self-signed cert.
	VIP6 net.IP
	// Same as VIP6, but as uint64
	VIP64 uint64

	EC256PrivateKey *ecdsa.PrivateKey
	// Secondary private keys.
	RSAPrivate *rsa.PrivateKey
	EDPrivate  *ed25519.PrivateKey

	// cached
	pub64 string

	Cert *x509.Certificate

	Authorized map[string]string

}

var certValidityPeriod = 100 * 365 * 24 * time.Hour

func NewAuth(cs ConfStore, name, domain string) *Auth{
	if name == "" {
		if os.Getenv("POD_NAME") != "" {
			name = os.Getenv("POD_NAME") + "." + os.Getenv("POD_NAMESPACE")
		} else {
			name, _ = os.Hostname()
		}
	}

	auth := &Auth{
		Config: cs,
		Name: name,
		Domain: domain,
	}

	// Use .ssh/ and the secondary config to load the keys.
	if cs != nil {
		err := auth.loadCert()
		if err != nil {
			log.Println("Error loading cert: ", err)
		}
	}

	if auth.EC256PrivateKey == nil {
		// Can't load the EC256 certs - generate new ones.
		auth.generateCert()
	}


	auth.VIP64 = auth.NodeIDUInt(auth.Pub)
	// Based on the primary EC256 key
	auth.pub64 = base64.RawURLEncoding.EncodeToString(auth.Pub)

	return auth
}

// From a key pair, generate a tls config with cert.
// Used for Auth and Client servers.
func (auth *Auth) GenerateTLSConfigServer() *tls.Config {
	var crt *tls.Certificate

	crt = &auth.tlsCerts[0]

	certs := []tls.Certificate{*crt}

	certMap := auth.GetCerts()
	certMap["*"] = crt

	return &tls.Config{
		Certificates: certs,
		NextProtos:   []string{"h2"},

		// Will only be called if client supplies SNI and Certificates empty
		GetCertificate: func(ch *tls.ClientHelloInfo) (*tls.Certificate, error) {
			// Log on each new TCP connection, after client hello
			//
			log.Printf("Server/NewConn/CH %s %v %v", ch.ServerName, ch.SupportedProtos, ch.Conn.RemoteAddr())
			// doesn't include :5228
			c, ok := certMap[ch.ServerName]
			if ok {
				return c, nil
			}
			return crt, nil
		},
	}
}

// Get all known certificates from the config store.
// "istio" is a special name, set if istio certs are found
//
func (auth *Auth) GetCerts() map[string]*tls.Certificate {
	certMap := map[string]*tls.Certificate{}

	// Attempt istio certs.
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
	for i := 0; i < 3; i++ {
		hasher := crypto.SHA256.New()
		hasher.Write(data) //[0:64]) // only public key, for debug
		hash := hasher.Sum(nil)

		r, s, _ := ecdsa.Sign(rand.Reader, auth.EC256PrivateKey, hash)

		copy(sig, r.Bytes())
		copy(sig[32:], s.Bytes())

		//log.Println("SND SIG: ", hex.EncodeToString(sig))
		//log.Println("SND PUB: ", hex.EncodeToString(data[len(data)-64:]))
		//log.Println("SND HASH: ", hex.EncodeToString(hash))
		//log.Printf("SND PAYLOAD: %d %s", len(data), hex.EncodeToString(data))
		err := Verify(data, auth.Pub[1:], sig)
		if err != nil {
			log.Println("Bad msg", err)
			log.Println("SIG: ", hex.EncodeToString(sig))
			log.Println("PUB: ", hex.EncodeToString(auth.Pub))
			log.Println("PRIV: ", hex.EncodeToString(auth.Priv))
			log.Println("HASH: ", hex.EncodeToString(hash))
		} else {
			return
		}
	}
}

func Verify(data []byte, pub []byte, sig []byte) error {
	hasher := crypto.SHA256.New()
	hasher.Write(data) //[0:64]) // only public key, for debug
	hash := hasher.Sum(nil)

	// Expects 0x4 prefix - we don't send the 4.
	//x, y := elliptic.Unmarshal(curve, pub)
	x := new(big.Int).SetBytes(pub[0:32])
	y := new(big.Int).SetBytes(pub[32:64])
	if !elliptic.P256().IsOnCurve(x, y) {
		return errors.New("Invalid public key")
	}

	pubKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	r := big.NewInt(0).SetBytes(sig[0:32])
	s := big.NewInt(0).SetBytes(sig[32:64])
	match := ecdsa.Verify(pubKey, hash, r, s)
	if match {
		return nil
	} else {
		//log.Printf("PAYLOAD: %d %s", len(data), hex.EncodeToString(data))

		//log.Println(pubKey)

		return errors.New("Failed to validate signature ")
	}
}


// Generate a config to be used in a HTTP client, using the primary identity and cert.
func (auth *Auth) GenerateTLSConfigClient() *tls.Config {
	// see transport.go in http onceSetNextProtoDefaults
	return &tls.Config{
		// VerifyPeerCertificate used instead
		InsecureSkipVerify: true,

		Certificates: auth.tlsCerts,
		// not set on client !! Setting it also disables Auth !
		//NextProtos: nextProtosH2,
	}
}

// Load the primary cert - expects a PEM key file
func (auth *Auth) loadCert() error {
	keyPEM, err := auth.Config.Get("ec-key.pem")
	if err != nil {
		return err
	}
	certPEM, err := auth.Config.Get("ec-cert.pem")
	if err != nil {
		return err
	}
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}

	pk := tlsCert.PrivateKey.(*ecdsa.PrivateKey)

	auth.tlsCerts = []tls.Certificate{tlsCert}
	auth.EC256PrivateKey = pk
	auth.Priv = pk.D.Bytes()
	auth.Pub = elliptic.Marshal(elliptic.P256(), pk.X, pk.Y) // starts with 0x04 == uncompressed curve

	auth.VIP6 = Pub2VIP(auth.Pub)

	// Attempt istio certs.
	if _, err := os.Stat("./etc/certs/key.pem"); !os.IsNotExist(err) {
		crt, err := tls.LoadX509KeyPair("./etc/certs/cert-chain.pem", "./etc/certs/key.pem")
		if err != nil {
			log.Println("Failed to load system istio certs", err)
		} else {
			auth.RSAPrivate = crt.PrivateKey.(*rsa.PrivateKey)
			auth.tlsCerts = append(auth.tlsCerts, crt)
			if crt.Leaf != nil {
				log.Println("Loaded istio cert ", crt.Leaf.URIs)
			}
		}
	}

	//keyRSA, err := auth.Config.Get(".ssh/id_rsa")
	//if err == nil {
	//	auth.setKey(keyRSA)
	//}
	//keyRSA, err = auth.Config.Get(".ssh/id_ed25519")
	//if err == nil {
	//	auth.setKey(keyRSA)
	//}
	return nil
}

// generateCert will generate the keys and populate the Pub/Priv fields.
// Will set privateKey, Priv, Pub
// Pub, Priv should be saved
func (auth *Auth) generateCert() {
	// d, x,y
	priv, x, y, err := elliptic.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal("Unexpected eliptic error")
	}

	pk := ecdsa.PublicKey{X: x, Y: y, Curve: elliptic.P256()}
	d := new(big.Int).SetBytes(priv[0:32])

	auth.Priv = priv

	auth.EC256PrivateKey = &ecdsa.PrivateKey{D: d, PublicKey: pk}
	auth.Pub = elliptic.Marshal(elliptic.P256(), x, y) // starts with 0x04 == uncompressed curve
	auth.PubKey = base64.RawURLEncoding.EncodeToString(auth.Pub)
	auth.VIP6 = Pub2VIP(auth.Pub)
	//b64 := base64.URLEncoding.WithPadding(base64.NoPadding)
	//
	//pub64 := b64.EncodeToString(pub)
	//priv64 := b64.EncodeToString(priv)

	if auth.Name == "" {
		auth.Name = base64.RawURLEncoding.EncodeToString(auth.NodeID())
	}
	keyPEM, certPEM := auth.generateAndSaveSelfSigned(auth.EC256PrivateKey, auth.Name+"."+auth.Domain)
	tlsCert, _ := tls.X509KeyPair(certPEM, keyPEM)
	auth.tlsCerts = []tls.Certificate{tlsCert}
}

var (
	MESH_NETWORK = []byte{0xFD, 0x00, 0x00, 0x00, 0x00, 0x00, 0, 0x00}
)

func (auth *Auth) NodeID() []byte {
	return auth.VIP6[8:]
}

// Generate and save the primary self-signed Certificate
func (auth *Auth) generateAndSaveSelfSigned(priv *ecdsa.PrivateKey, sans ...string) ([]byte, []byte) {
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
		IPAddresses:           []net.IP{auth.VIP6},
	}

	// Sign with the private key.

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		panic(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	ecb, _ := x509.MarshalECPrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecb})

	if auth.Config != nil {
		auth.Config.Set("ec-key.pem", keyPEM)
		auth.Config.Set("ec-cert.pem", certPEM)
		//pub64 := base64.StdEncoding.EncodeToString(auth.Pub)
		//sshPub := "ecdsa-sha2-nistp256 " + SSH_ECPREFIX + pub64 + " " + auth.Name + "@" + auth.Domain
		//auth.Config.Set("id_ecdsa.pub", []byte(sshPub))
	}
	return keyPEM, certPEM
}


// Convert a public key to a VIP. This is the primary ID of the nodes.
// Primary format is the 64-byte EC256 public key.
//
// For RSA, the ASN.1 format of the byte[] is used.
// For ED, the 32-byte raw encoding.
func Pub2VIP(pub []byte) net.IP {
	ip6 := make([]byte, 16)
	copy(ip6, MESH_NETWORK)

	binary.BigEndian.PutUint64(ip6[8:], Pub2ID(pub))
	return net.IP(ip6)
}


func (auth *Auth) NodeIDUInt(pub []byte) uint64 {
	return Pub2ID(pub)
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


func RawToCertChain(rawCerts [][]byte) ([]*x509.Certificate, error){
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
// The key is in the marshalled format - use KeyBytes to convert a crypto.PublicKey.
//
func (auth *Auth) Auth(key []byte, role string) string {
	roles := auth.Authorized[string(key)]

	return roles
}

func KeyToCertificate(sk crypto.PrivateKey) (*tls.Certificate, error) {

	//certKeyPub, err := x509.MarshalPKIXPublicKey(certKey.Public())
	//if err != nil {
	//	return nil, err
	//}
	//signature, err := sk.Sign(append([]byte(certificatePrefix), certKeyPub...))
	//if err != nil {
	//	return nil, err
	//}
	//value, err := asn1.Marshal(signedKey{
	//	PubKey:    keyBytes,
	//	Signature: signature,
	//})
	//if err != nil {
	//	return nil, err
	//}

	sn, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: sn,
		NotBefore:    time.Time{},
		NotAfter:     time.Now().Add(certValidityPeriod),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl,
		PublicKey(sk), sk)
	if err != nil {
		return nil, err
	}

	return &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  sk,
	}, nil
}

//// We want nodes without AES hardware (e.g. ARM) support to always use ChaCha.
//// Only if both nodes have AES hardware support (e.g. x86), AES should be used.
//// x86->x86: AES, ARM->x86: ChaCha, x86->ARM: ChaCha and ARM->ARM: Chacha
//// This function returns true if we don't have AES hardware support, and false otherwise.
//// Thus, ARM servers will always use their own cipher suite preferences (ChaCha first),
//// and x86 servers will aways use the client's cipher suite preferences.
//func preferServerCipherSuites() bool {
//	// Copied from the Go TLS implementation.
//
//	// Check the cpu flags for each platform that has optimized GCM implementations.
//	// Worst case, these variables will just all be false.
//	var (
//		hasGCMAsmAMD64 = cpu.X86.HasAES && cpu.X86.HasPCLMULQDQ
//		hasGCMAsmARM64 = cpu.ARM64.HasAES && cpu.ARM64.HasPMULL
//		// Keep in sync with crypto/aes/cipher_s390x.go.
//		hasGCMAsmS390X = cpu.S390X.HasAES && cpu.S390X.HasAESCBC && cpu.S390X.HasAESCTR && (cpu.S390X.HasGHASH || cpu.S390X.HasAESGCM)
//
//		hasGCMAsm = hasGCMAsmAMD64 || hasGCMAsmARM64 || hasGCMAsmS390X
//	)
//	return !hasGCMAsm
//}

func IDFromPublicKey(key crypto.PublicKey) string {
	m := MarshalPublicKey(key)
	return base64.RawURLEncoding.EncodeToString(m)
}

func GetOrGenerateCert(fname string, priv crypto.PrivateKey) ([]byte, error) {
	if fname != "" {
		ex, err := ioutil.ReadFile(fname)
		if err == nil {
			certPEMBlock, _ := pem.Decode(ex)
			return certPEMBlock.Bytes, nil
			//tlsCert, err := x509.ParseCertificates(ex)
			//if err != nil {
			//	return nil, err
			//}
			//
			//return tlsCert, nil
		}
	}
	crt, err := KeyToCertificate(priv)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: crt.Certificate[0]})

	if fname != "" {
		err = ioutil.WriteFile(fname, certPEM, 0700)
	}
	return crt.Certificate[0], err
}

func GetOrGenerateKey(fname string) (crypto.PrivateKey, error) {
	if fname != "" {
		ex, err := ioutil.ReadFile(fname)
		if err == nil {
			key, err := UnmarshalPrivateKey(ex)
			if err != nil {
				return nil, err
			}
			return key, nil
		}
	}
	key := GenerateKeyPair()
	data, _ := MarshalPrivateKey(key)
	var err error
	if fname != "" {
		err = ioutil.WriteFile(fname, data, 0700)
	}
	return key, err
}

func GenerateKeyPair() ed25519.PrivateKey {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	return priv
}

func GenerateRSAKeyPair() *rsa.PrivateKey {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	return priv
}

func GenerateEC256KeyPair() *ecdsa.PrivateKey {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return priv
}

// Serialize a privte key. Will use the SSH format, for more interop.
// A lot of apps use SSH public keys for access.
// Supports RSA, EC256 and ED25519
func MarshalPrivateKey(key crypto.PrivateKey) ([]byte, error) {
	if k, ok := key.(ed25519.PrivateKey); ok {
		return []byte(k), nil
	}
	if k, ok := key.(*ecdsa.PrivateKey); ok {
		return x509.MarshalECPrivateKey(k)
	}
	if k, ok := key.(*rsa.PrivateKey); ok {
		bk := x509.MarshalPKCS1PrivateKey(k)
		return bk, nil
	}

	return nil, errors.New("unknown key")
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

// Extract the key from the bytes.
// IPFS uses a protobuf, with keytype plus 'raw' encoding.
// The raw encoding is the 'natural' format.
func UnmarshalPrivateKey(rawKey []byte) (key crypto.PrivateKey, err error) {
	if len(rawKey) == 64 {
		key = ed25519.PrivateKey(rawKey)
		// public is the last 32 bytes
		return
	}

	if len(rawKey) < 256 {
		// der size: ~120 bytes ( including version(1), PrivateKey(32) curve oid(7), public key 65 bytes )
		key, err = x509.ParseECPrivateKey(rawKey)
		return key, err
	}

	// der size: ~1200 bytes
	key, err = x509.ParsePKCS1PrivateKey(rawKey)
	return
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

