package xds

import (
	"fmt"
	"log"
	"net"
	"testing"

	"crypto/tls"
	"flag"

	msgs "github.com/costinm/ugate/webpush"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	testAddr = flag.String("test.addr", "", "URL of tested server, empty for in-process")
)

func startLocalServer(t *testing.T) (*grpc.Server, string) {
	// https://github.com/grpc/grpc-go/blob/master/test/end2end_test.go
	sopts := []grpc.ServerOption{grpc.MaxConcurrentStreams(64)}
	sopts = append(sopts, grpc.MaxMsgSize(8192))

	kp, err := tls.X509KeyPair([]byte(CERT), []byte(PRIV))
	credOpt := credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{kp}})
	/*
		credOpt, err := credentials.NewServerTLSFromFile("../testdata/server1.pem", "../testdata/server1.key")
		if err != nil {
			t.Fatal("Failed to load", err)
		}*/

	sopts = append(sopts, grpc.Creds(credOpt))

	la := "localhost:0"
	lis, err := net.Listen("tcp", la)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer(sopts...)

	t.Log(lis.Addr().String())
	addr := lis.Addr().String()

	wp := &GrpcService{
		Mux: msgs.DefaultMux,
	}
	RegisterAggregatedDiscoveryServiceServer(s, wp)
	go s.Serve(lis)

	return s, addr
}

func TestNativeXDS(t *testing.T) {

}

func TestGRpc(t *testing.T) {
	addr := *testAddr
	if len(*testAddr) == 0 {
		s, a := startLocalServer(t)
		addr = a
		fmt.Println(a)
		defer s.Stop()
	}

	conn, client, err := Connect(addr, CLIENT_PEM)
	defer conn.Close()

	res, err := client.StreamAggregatedResources(context.Background())
	if err != nil {
		log.Fatal("Subscribe fail ", err)
	}
	log.Println("Response", res)

	go func() {
		for {
			msg, err := res.Recv()
			if err != nil {
				t.Fatal("Error in receive", err)
			}
			t.Log("Received ", msg)
		}
	}()

	res.Send(&Request{})
}

const (
	// This is a cert pool - copied from grpc tests.
	// should include the self-signed cert.
	CLIENT_PEM = `-----BEGIN CERTIFICATE-----
MIICSjCCAbOgAwIBAgIJAJHGGR4dGioHMA0GCSqGSIb3DQEBCwUAMFYxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIEwpTb21lLVN0YXRlMSEwHwYDVQQKExhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQxDzANBgNVBAMTBnRlc3RjYTAeFw0xNDExMTEyMjMxMjla
Fw0yNDExMDgyMjMxMjlaMFYxCzAJBgNVBAYTAkFVMRMwEQYDVQQIEwpTb21lLVN0
YXRlMSEwHwYDVQQKExhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQxDzANBgNVBAMT
BnRlc3RjYTCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAwEDfBV5MYdlHVHJ7
+L4nxrZy7mBfAVXpOc5vMYztssUI7mL2/iYujiIXM+weZYNTEpLdjyJdu7R5gGUu
g1jSVK/EPHfc74O7AyZU34PNIP4Sh33N+/A5YexrNgJlPY+E3GdVYi4ldWJjgkAd
Qah2PH5ACLrIIC6tRka9hcaBlIECAwEAAaMgMB4wDAYDVR0TBAUwAwEB/zAOBgNV
HQ8BAf8EBAMCAgQwDQYJKoZIhvcNAQELBQADgYEAHzC7jdYlzAVmddi/gdAeKPau
sPBG/C2HCWqHzpCUHcKuvMzDVkY/MP2o6JIW2DBbY64bO/FceExhjcykgaYtCH/m
oIU63+CFOTtR7otyQAWHqXa7q4SbCDlG7DyRFxqG0txPtGvy12lgldA2+RgcigQG
Dfcog5wrJytaQ6UA0wE=
-----END CERTIFICATE-----

	`
	// self-signed cert
	CERT = `-----BEGIN CERTIFICATE-----
MIICnDCCAgWgAwIBAgIBBzANBgkqhkiG9w0BAQsFADBWMQswCQYDVQQGEwJBVTET
MBEGA1UECBMKU29tZS1TdGF0ZTEhMB8GA1UEChMYSW50ZXJuZXQgV2lkZ2l0cyBQ
dHkgTHRkMQ8wDQYDVQQDEwZ0ZXN0Y2EwHhcNMTUxMTA0MDIyMDI0WhcNMjUxMTAx
MDIyMDI0WjBlMQswCQYDVQQGEwJVUzERMA8GA1UECBMISWxsaW5vaXMxEDAOBgNV
BAcTB0NoaWNhZ28xFTATBgNVBAoTDEV4YW1wbGUsIENvLjEaMBgGA1UEAxQRKi50
ZXN0Lmdvb2dsZS5jb20wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGBAOHDFSco
LCVJpYDDM4HYtIdV6Ake/sMNaaKdODjDMsux/4tDydlumN+fm+AjPEK5GHhGn1Bg
zkWF+slf3BxhrA/8dNsnunstVA7ZBgA/5qQxMfGAq4wHNVX77fBZOgp9VlSMVfyd
9N8YwbBYAckOeUQadTi2X1S6OgJXgQ0m3MWhAgMBAAGjazBpMAkGA1UdEwQCMAAw
CwYDVR0PBAQDAgXgME8GA1UdEQRIMEaCECoudGVzdC5nb29nbGUuZnKCGHdhdGVy
em9vaS50ZXN0Lmdvb2dsZS5iZYISKi50ZXN0LnlvdXR1YmUuY29thwTAqAEDMA0G
CSqGSIb3DQEBCwUAA4GBAJFXVifQNub1LUP4JlnX5lXNlo8FxZ2a12AFQs+bzoJ6
hM044EDjqyxUqSbVePK0ni3w1fHQB5rY9yYC5f8G7aqqTY1QOhoUk8ZTSTRpnkTh
y4jjdvTZeLDVBlueZUTDRmy2feY5aZIU18vFDK08dTG0A87pppuv1LNIR3loveU8
-----END CERTIFICATE-----
`

	// private (and public ) RSA keys
	PRIV = `-----BEGIN PRIVATE KEY-----
MIICdQIBADANBgkqhkiG9w0BAQEFAASCAl8wggJbAgEAAoGBAOHDFScoLCVJpYDD
M4HYtIdV6Ake/sMNaaKdODjDMsux/4tDydlumN+fm+AjPEK5GHhGn1BgzkWF+slf
3BxhrA/8dNsnunstVA7ZBgA/5qQxMfGAq4wHNVX77fBZOgp9VlSMVfyd9N8YwbBY
AckOeUQadTi2X1S6OgJXgQ0m3MWhAgMBAAECgYAn7qGnM2vbjJNBm0VZCkOkTIWm
V10okw7EPJrdL2mkre9NasghNXbE1y5zDshx5Nt3KsazKOxTT8d0Jwh/3KbaN+YY
tTCbKGW0pXDRBhwUHRcuRzScjli8Rih5UOCiZkhefUTcRb6xIhZJuQy71tjaSy0p
dHZRmYyBYO2YEQ8xoQJBAPrJPhMBkzmEYFtyIEqAxQ/o/A6E+E4w8i+KM7nQCK7q
K4JXzyXVAjLfyBZWHGM2uro/fjqPggGD6QH1qXCkI4MCQQDmdKeb2TrKRh5BY1LR
81aJGKcJ2XbcDu6wMZK4oqWbTX2KiYn9GB0woM6nSr/Y6iy1u145YzYxEV/iMwff
DJULAkB8B2MnyzOg0pNFJqBJuH29bKCcHa8gHJzqXhNO5lAlEbMK95p/P2Wi+4Hd
aiEIAF1BF326QJcvYKmwSmrORp85AkAlSNxRJ50OWrfMZnBgzVjDx3xG6KsFQVk2
ol6VhqL6dFgKUORFUWBvnKSyhjJxurlPEahV6oo6+A+mPhFY8eUvAkAZQyTdupP3
XEFQKctGz+9+gKkemDp7LBBMEMBXrGTLPhpEfcjv/7KPdnFHYmhYeBTBnuVmTVWe
F98XJ7tIFfJq
-----END PRIVATE KEY-----

	`
)
