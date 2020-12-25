package main

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/costinm/ugate"
)


func main() {
	// By default, pass through using net.Dialer
	ug := ugate.NewGate(&net.Dialer{})

	ug.DefaultPorts(15000)

	// direct TCP connect to local iperf3 and fortio (or HTTP on default port)
	ug.Add(&ugate.ListenerConf{
		Port: 15101,
		Remote: "localhost:5201",
	})
	ug.Add(&ugate.ListenerConf{
		Port: 15108,
		Remote: "localhost:8080",
	})
	key := ugate.GenerateKeyPair()
	cert, _ := ugate.KeyToCertificate(key)

	ug.Add(&ugate.ListenerConf{
		Port: 15102,
		Protocol: "tls",
		TLSConfig: &tls.Config {
				MinVersion:               tls.VersionTLS13,
				//PreferServerCipherSuites: ugate.preferServerCipherSuites(),
				InsecureSkipVerify:       true, // This is not insecure here. We will verify the cert chain ourselves.
				ClientAuth:               tls.RequestClientCert, // not require - we'll fallback to JWT
				Certificates:             []tls.Certificate{*cert},
				VerifyPeerCertificate: func(_ [][]byte, _ [][]*x509.Certificate) error {
					panic("tls config not specialized for peer")
				},
				NextProtos:             []string{"h2","spdy","wss"},
				//SessionTicketsDisabled: true,
		},
		//Remote: "localhost:4444",
		Remote: "localhost:5201",
	})
	ug.Add(&ugate.ListenerConf{
		Port: 15103,
		Protocol: "tcp",
		Remote: "localhost:15102", // The TLS server
		RemoteTLS: &tls.Config{
			MinVersion:               tls.VersionTLS13,
			//PreferServerCipherSuites: ugate.preferServerCipherSuites(),
			InsecureSkipVerify:       true, // This is not insecure here. We will verify the cert chain ourselves.
			ClientAuth:               tls.RequestClientCert, // not require - we'll fallback to JWT
			Certificates:             []tls.Certificate{*cert},
			VerifyPeerCertificate: func(_ [][]byte, _ [][]*x509.Certificate) error {
				panic("tls config not specialized for peer")
			},
			NextProtos:             []string{"h2","spdy","wss"},
			//SessionTicketsDisabled: true,
		},
	})
	ug.Add(&ugate.ListenerConf{
		Port: 15104,
		Protocol: "tcp",
		Remote: "localhost:15101",
	})

	log.Println("Started debug on 15020, UID/GID", os.Getuid(), os.Getegid())
	err := http.ListenAndServe(":15020", nil)
	if err != nil {
		log.Fatal(err)
	}
}

