package test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"github.com/costinm/meshauth"
	"github.com/costinm/ssh-mesh/nio"
	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/echo"
)

// Separate package to avoid recursive deps.
// The ugated package depends on each protocol impl.
// This is used from each protocol dir for single-protocol tests.
// Also an example on how to use without configs.

// NewClientNode inits a test node from a config dir, with no default listeners.
func NewClientNode(maCfg *meshauth.MeshCfg, cfg *ugate.MeshSettings) (*ugate.UGate, error) {
	ma := meshauth.NewMeshAuth(maCfg)

	// This is a base gate - without any special protocol (from ugated).
	ug := ugate.New(ma, cfg)
	ug.ListenerProto["echo"] = echo.EchoPortHandler

	err := ug.Start()
	return ug, err
}

// NewTestNode creates a node with the given config.
func NewTestNode(acfg *meshauth.MeshCfg, cfg *ugate.MeshSettings) *ugate.UGate {
	basePort := cfg.BasePort

	a := meshauth.NewMeshAuth(acfg)
	if a.Priv == "" {
		a.InitSelfSigned("")
	}
	ug := ugate.New(a, cfg)

	ech := &echo.EchoHandler{UGate: ug}
	ug.Mux.Handle("/debug/echo/", ech)

	ug.ListenerProto["echo"] = echo.EchoPortHandler

	// Echo - TCP
	ug.StartListener(&meshauth.PortListener{
		Address:  fmt.Sprintf("0.0.0.0:%d", basePort+11),
		Protocol: "echo", // TLS ?
	})
	ug.StartListener(&meshauth.PortListener{
		Address:  fmt.Sprintf("0.0.0.0:%d", basePort+12),
		Protocol: "echo",
	})

	return ug
}

func InitEcho(port int) *ugate.UGate {
	//cs := meshauth.NewConf()
	ug := ugate.New(nil, &ugate.MeshSettings{
		BasePort: port,
	})

	// Echo - TCP

	ug.ListenerProto["echo"] = echo.EchoPortHandler

	ug.StartListener(&meshauth.PortListener{
		Address:  fmt.Sprintf("0.0.0.0:%d", port+12),
		Protocol: "echo",
	})
	return ug
}

var chunk1 = []byte("Hello world")
var chunk2 = []byte("chunk2")

// CheckEcho will verify the behavior of the echo server.
// - write chunk1 (to verify we don't hang )
// - read response from server  - should include metadata for the request plus chunk1
// -
func CheckEcho(in io.Reader, out io.Writer) (string, error) {

	d := make([]byte, 2048)
	// RoundTripStart with a write - client send first (echo will wait, to verify that body is not cached)
	_, err := out.Write(chunk1)
	if err != nil {
		return "", err
	}

	//ab.SetDeadline(time.Now().StartListener(5 * time.Second))
	n, err := in.Read(d)
	if err != nil {
		return "", err
	}

	idx := bytes.IndexByte(d[0:n], '\n')
	if idx < 0 {
		return string(d[0:n]), errors.New("missing header")
	}
	js := string(d[0:idx])
	idx++ // skip \n
	// Server writes 2 chunkes - the header and what we wrote. There is a flush, but
	// the transports don't guarantee boundary.
	start := 0
	if idx < n {
		copy(d[0:], d[idx:n])
		start = n - idx
		n = start
	} else {
		n = 0
	}
	if n < len(chunk1) {
		n, err = in.Read(d[start:])
		if err != nil {
			return "", err
		}
		n += start
	}

	if !bytes.Equal(chunk1, d[0:n]) {
		return js, errors.New("miss-matched result1 " + string(d[0:n]))
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
	if cw, ok := out.(nio.CloseWriter); ok {
		cw.CloseWrite()
	} else {
		out.(io.Closer).Close()
	}
	// Possible issue: the server has sent FIN, but reader (h3 response body) did not
	// receive it.

	n, err = in.Read(d)
	if err != io.EOF {
		return "", err
	}

	return js, nil
}

var AliceMeshAuthCfg = &meshauth.MeshCfg{
	Name: "alice",
	Domain: "test.m.internal",
	Priv: `
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIOa4YuOsiCIqcFqqknjeTaPCwVEuF/X9YJMdf77V09HyoAoGCCqGSM49
AwEHoUQDQgAE1ac88a5eI83WDHmtOJmce4HhRO3s9iObYKnP9QkpZPMRYwJcvi0i
N9miG3v5LoooDq6r9Nt2e/koHw6E1SvmlQ==
-----END EC PRIVATE KEY-----
`,
	CertBytes: `
-----BEGIN CERTIFICATE-----
MIIBeDCCAR+gAwIBAgIQUzvKlOHbjBotS03Lku+pBTAKBggqhkjOPQQDAjAXMQkw
BwYDVQQKEwAxCjAIBgNVBAMTAS4wHhcNMjQwMTA1MTg0NDMyWhcNMjUwMTA0MTg0
NDMyWjAXMQkwBwYDVQQKEwAxCjAIBgNVBAMTAS4wWTATBgcqhkjOPQIBBggqhkjO
PQMBBwNCAATVpzzxrl4jzdYMea04mZx7geFE7ez2I5tgqc/1CSlk8xFjAly+LSI3
2aIbe/kuiigOrqv023Z7+SgfDoTVK+aVo00wSzAOBgNVHQ8BAf8EBAMCBaAwHQYD
VR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwDAYDVR0R
BAUwA4IBLjAKBggqhkjOPQQDAgNHADBEAiAGfrW7fgnGYV0AHPhc2sgymzpuy0DS
T8kswXTqlc/b2gIgUNTQoHbS9R/VTrxIbHCQeL5TZE4XCGq3DQ9lHgAld4E=
-----END CERTIFICATE-----
`,
}

var BobMeshAuthCfg = &meshauth.MeshCfg{
	Name: "bob",
	Domain: "test.m.internal",
	Priv: `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIG0y/ACuY0grzpMCZRWs/mUCgXY/vaMFN5MZOY+X901IoAoGCCqGSM49
AwEHoUQDQgAE51wdYyA7nhXKgUzvruRo4ZchJLNgJTlSkKRLpNYHhLrBjHivC4A4
HgysDlU4frwsTSg6qPOiXkTkA8VZJzdHMg==
-----END EC PRIVATE KEY-----`,
	CertBytes: `-----BEGIN CERTIFICATE-----
MIIBejCCASCgAwIBAgIRAK9oRInSJbAZDLpCWQX/UN8wCgYIKoZIzj0EAwIwFzEJ
MAcGA1UEChMAMQowCAYDVQQDEwEuMB4XDTIyMTEwNDE0MjgxMFoXDTIzMTEwNDE0
MjgxMFowFzEJMAcGA1UEChMAMQowCAYDVQQDEwEuMFkwEwYHKoZIzj0CAQYIKoZI
zj0DAQcDQgAE51wdYyA7nhXKgUzvruRo4ZchJLNgJTlSkKRLpNYHhLrBjHivC4A4
HgysDlU4frwsTSg6qPOiXkTkA8VZJzdHMqNNMEswDgYDVR0PAQH/BAQDAgWgMB0G
A1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMAwGA1Ud
EQQFMAOCAS4wCgYIKoZIzj0EAwIDSAAwRQIhALQoenr80BTUkIeFKyqOJTQM75sx
31/cQyVaS0hZfTRsAiBPnuyNeeLFuQ7+2ogMB2FQsg8oIjFEcd781XFEjJWMDg==
-----END CERTIFICATE-----
`,
}

// Helpers for setting up a ugate test env.

// Verify using the basic echo server.
func EchoClient2(t *testing.T, lout io.WriteCloser, lin io.Reader, serverFirst bool) {
	b := make([]byte, 1024)
	timer := time.AfterFunc(3*time.Second, func() {
		log.Println("timeout")
		//lin.CloseWithError(errors.New("timeout"))
		lout.Close() // (errors.New("timeout"))
	})

	if serverFirst {
		b := make([]byte, 1024)
		n, err := lin.Read(b)
		if n == 0 || err != nil {
			t.Fatal(n, err)
		}
	}

	lout.Write([]byte("Ping"))
	n, err := lin.Read(b)
	if n != 4 {
		t.Error(n, err)
	}
	timer.Stop()
}
