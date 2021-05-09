package test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/cfgfs"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

// WIP: The e2e tests should work with either 'external' (already setup) servers or start
// an equivalent set. The config and keys in this file are matching the
// cmd/ugate/testdata/{alice,bob,carol}.
// If tests need the status, they should use the debug endpoints.
// For events, they should use the messaging endpoints.
//
// Some tests may start additional uGates for internal functionality testing.
//

// WIP: updating the ports and scenarios
var (
	// Base port 6000
	// Uses H2R to connect to Gate
	Alice *ugatesvc.UGate

	// 6100
	// Uses QUIC to connect to Gate
	Bob *ugatesvc.UGate

	// Client - in process.
	// 6200
	Carol *ugatesvc.UGate

	// Gateway/control plane for Bob and Alice
	// 6300
	Gate *ugatesvc.UGate

	// Echo - acts as a 'server' providing the echo service.
	// 6400
	Echo *ugatesvc.UGate
)


func InitEcho(port int) *ugatesvc.UGate {
	cs := cfgfs.NewConf()
	ug := ugatesvc.NewGate(&net.Dialer{}, nil, &ugate.GateCfg{
		BasePort: port,
	}, cs)

	// Echo - TCP
	ug.Add(&ugate.Listener{
		Address:   fmt.Sprintf("0.0.0.0:%d", port + 12),
		Handler:   &ugatesvc.EchoHandler{},
	})
	ug.Mux.Handle("/", &ugatesvc.EchoHandler{})
	return ug
}

// InitTestServer creates a node with the given config.
func InitTestServer(kubecfg string, cfg *ugate.GateCfg, ext func(*ugatesvc.UGate)) *ugatesvc.UGate {
	basePort := cfg.BasePort
	cfg.Domain = "test.cluster.local"
	cs := cfgfs.NewConf()
	cs.Set("secret/" + cfg.Name + "." + cfg.Domain, []byte(kubecfg))

	ug := ugatesvc.NewGate(&net.Dialer{}, nil, cfg, cs)

	// Similar with the cmd, add extensions to avoid deps in core.
	if ext != nil {
		ext(ug)
	}

	// Echo - TCP
	ug.Add(&ugate.Listener{
		Address:   fmt.Sprintf("0.0.0.0:%d", basePort+11),
		Protocol:  "tls",
		Handler:   &ugatesvc.EchoHandler{},
	})
	ug.Add(&ugate.Listener{
		Address: fmt.Sprintf("0.0.0.0:%d", basePort+12),
		Handler: &ugatesvc.EchoHandler{},
	})
	ug.Mux.Handle("/", &ugatesvc.EchoHandler{})
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
	if cw, ok := out.(ugate.CloseWriter); ok {
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


// The tests will use 3 fixed identies, to keep things simple.

// Alice runs on 14000 ( testdata/alice for standalone debug ).
const ALICE_ID="A3UCXD63FCFXMX7GH64FZM5EAHH3PGLKWRMBHPGY4AA3MGM6SXPQ"
const ALICE_PORT=6007
const ALICE_VIP = "fd00::f054:f1ab:89ed:c146"

const BOB_ID="BVMXJRUH7FVKYBZBXJ7HQVHDIMDO7ADRUUQLYMDU6X7SARNP5OXA"
const BOB_PORT=6107
const BOB_VIP="fd00::156:6388:1fdf:cb69"

const CAROL_PORT=6207
const CAROL_ID="IN7E4J4VZ66TJZFY4ZABEM6STXIBEU7GVAVOE5NCM5DLVRIDJNFQ"
const CAROL_VIP="fd00::e524:71c0:d68e:f0ce"

const ALICE_KEYS = `
{
  "apiVersion": "v1",
  "kind": "Config",
  "clusters": [],
  "users": [
    {
      "name": "A3UCXD63FCFXMX7GH64FZM5EAHH3PGLKWRMBHPGY4AA3MGM6SXPQ",
      "user": {
        "client-certificate-data": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUIwekNDQVhtZ0F3SUJBZ0lSQUxRSWhRdytlL1h2RGhabjFjZkxwZkF3Q2dZSUtvWkl6ajBFQXdJd09URVcKTUJRR0ExVUVDaE1OYUM1M1pXSnBibVl1YVc1bWJ6RWZNQjBHQTFVRUF4TVdZMjl6ZEdsdU1UWXVhQzUzWldKcApibVl1YVc1bWJ6QWVGdzB5TVRBME1UVXhNekUwTXpsYUZ3MHlNakEwTVRVeE16RTBNemxhTURreEZqQVVCZ05WCkJBb1REV2d1ZDJWaWFXNW1MbWx1Wm04eEh6QWRCZ05WQkFNVEZtTnZjM1JwYmpFMkxtZ3VkMlZpYVc1bUxtbHUKWm04d1dUQVRCZ2NxaGtqT1BRSUJCZ2dxaGtqT1BRTUJCd05DQUFUU0ROL1UwZEk3UGpIdWx1V3BSdzR5akxmagpJblZiNE9uSEVRTGlVTkVUZW94SlZXQUVzRitCamo5Yzk2RU1YRElibitVZ3cvYVRldkJVOGF1SjdjRkdvMkl3CllEQU9CZ05WSFE4QkFmOEVCQU1DQmFBd0hRWURWUjBsQkJZd0ZBWUlLd1lCQlFVSEF3RUdDQ3NHQVFVRkJ3TUMKTUF3R0ExVWRFd0VCL3dRQ01BQXdJUVlEVlIwUkJCb3dHSUlXWTI5emRHbHVNVFl1YUM1M1pXSnBibVl1YVc1bQpiekFLQmdncWhrak9QUVFEQWdOSUFEQkZBaUJWcGNQUG94cjV4ZS8vSUdCcUVLVUsvZEhXNXlvc203RFdPNFN0CndVRkFaQUloQUp3NjExSXNBMmN1NFpQczdSQ2NmQ29EUlh2UWpQeUZQV1VZSzFFT1dOdTIKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=",
        "client-key-data": "LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JR0hBZ0VBTUJNR0J5cUdTTTQ5QWdFR0NDcUdTTTQ5QXdFSEJHMHdhd0lCQVFRZ2FmWGdCYzkwVVBGNWJ4QjMKeC9JMWFTUXdGVkR0OXFlSmVIUjRLQ0NPUmlTaFJBTkNBQVRTRE4vVTBkSTdQakh1bHVXcFJ3NHlqTGZqSW5WYgo0T25IRVFMaVVORVRlb3hKVldBRXNGK0JqajljOTZFTVhESWJuK1Vndy9hVGV2QlU4YXVKN2NGRwotLS0tLUVORCBQUklWQVRFIEtFWS0tLS0tCg=="
      }
    }
  ],
  "contexts": [
    {
      "name": "default",
      "context": {
        "cluster": "default",
        "user": "A3UCXD63FCFXMX7GH64FZM5EAHH3PGLKWRMBHPGY4AA3MGM6SXPQ"
      }
    }
  ],
  "current-context": "default"
}
`

const BOB_KEYS = `
{
  "apiVersion": "v1",
  "kind": "Config",
  "clusters": [],
  "users": [
    {
      "name": "BVMXJRUH7FVKYBZBXJ7HQVHDIMDO7ADRUUQLYMDU6X7SARNP5OXA",
      "user": {
        "client-certificate-data": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUIwekNDQVhpZ0F3SUJBZ0lRS1RXUlp4WWUyTzIxa2doRkNnL1Z0VEFLQmdncWhrak9QUVFEQWpBNU1SWXcKRkFZRFZRUUtFdzF0TG5kbFltbHVaaTVwYm1adk1SOHdIUVlEVlFRREV4WmpiM04wYVc0eE5pNXRMbmRsWW1sdQpaaTVwYm1adk1CNFhEVEl4TURReU5ERXpORGt3TlZvWERUSXlNRFF5TkRFek5Ea3dOVm93T1RFV01CUUdBMVVFCkNoTU5iUzUzWldKcGJtWXVhVzVtYnpFZk1CMEdBMVVFQXhNV1kyOXpkR2x1TVRZdWJTNTNaV0pwYm1ZdWFXNW0KYnpCWk1CTUdCeXFHU000OUFnRUdDQ3FHU000OUF3RUhBMElBQlBWUUVCRjBMMWZnTzlrQVpUS0RwdzZ5ZlI5WgpzWVh1dGJwY0trTUhoV3BJTkw0SDFxcDgxc0VOZ21KbmdIbnA4VU50NWxvbHA3VU5BVlpqaUIvZnkybWpZakJnCk1BNEdBMVVkRHdFQi93UUVBd0lGb0RBZEJnTlZIU1VFRmpBVUJnZ3JCZ0VGQlFjREFRWUlLd1lCQlFVSEF3SXcKREFZRFZSMFRBUUgvQkFJd0FEQWhCZ05WSFJFRUdqQVlnaFpqYjNOMGFXNHhOaTV0TG5kbFltbHVaaTVwYm1adgpNQW9HQ0NxR1NNNDlCQU1DQTBrQU1FWUNJUUMrbytLbDZ4cklaNEtUeVNTQWczd0p4L2pZOFFzWGlOb2VUZ1lrCmdYeTl4QUloQVBORlpHRm44UTBiMkFuSUI4LzFHR0Z0bzcwT3VWczF3cG1qRzByK3lQZnAKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=",
        "client-key-data": "LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JR0hBZ0VBTUJNR0J5cUdTTTQ5QWdFR0NDcUdTTTQ5QXdFSEJHMHdhd0lCQVFRZ3o1YXFiMmhIUDlkcjQ5QXMKMXRuM0Vwclg0TmdSbHQ1QS9qL1d2cjIxY2p1aFJBTkNBQVQxVUJBUmRDOVg0RHZaQUdVeWc2Y09zbjBmV2JHRgo3clc2WENwREI0VnFTRFMrQjlhcWZOYkJEWUppWjRCNTZmRkRiZVphSmFlMURRRldZNGdmMzh0cAotLS0tLUVORCBQUklWQVRFIEtFWS0tLS0tCg=="
      }
    }
  ],
  "contexts": [
    {
      "name": "default",
      "context": {
        "cluster": "default",
        "user": "BVMXJRUH7FVKYBZBXJ7HQVHDIMDO7ADRUUQLYMDU6X7SARNP5OXA"
      }
    }
  ],
  "current-context": "default"
}
`

const CAROL_KEYS = `
{
  "apiVersion": "v1",
  "kind": "Config",
  "clusters": [],
  "users": [
    {
      "name": "default",
      "user": {
        "client-certificate-data": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUIxRENDQVhtZ0F3SUJBZ0lSQU5zVzZ2ZHMzc051WEZkc3FYODRzNFV3Q2dZSUtvWkl6ajBFQXdJd09URVcKTUJRR0ExVUVDaE1OYUM1M1pXSnBibVl1YVc1bWJ6RWZNQjBHQTFVRUF4TVdZMjl6ZEdsdU1UWXVhQzUzWldKcApibVl1YVc1bWJ6QWVGdzB5TVRBME1UVXhNekV4TWpCYUZ3MHlNakEwTVRVeE16RXhNakJhTURreEZqQVVCZ05WCkJBb1REV2d1ZDJWaWFXNW1MbWx1Wm04eEh6QWRCZ05WQkFNVEZtTnZjM1JwYmpFMkxtZ3VkMlZpYVc1bUxtbHUKWm04d1dUQVRCZ2NxaGtqT1BRSUJCZ2dxaGtqT1BRTUJCd05DQUFUalVId0ZzZmh5VjlsUGJOU3RqYThod3RCQQpTQS9QQVlyM3lBSTZLTlppS0tqVTQyVjZyOWJOaDN4R2xMYmJOcDdVbis4NmJEZ2p3ZVVrY2NEV2p2RE9vMkl3CllEQU9CZ05WSFE4QkFmOEVCQU1DQmFBd0hRWURWUjBsQkJZd0ZBWUlLd1lCQlFVSEF3RUdDQ3NHQVFVRkJ3TUMKTUF3R0ExVWRFd0VCL3dRQ01BQXdJUVlEVlIwUkJCb3dHSUlXWTI5emRHbHVNVFl1YUM1M1pXSnBibVl1YVc1bQpiekFLQmdncWhrak9QUVFEQWdOSkFEQkdBaUVBNWkwZXZ1TTU0M1hVVldkOFhpRjRrVDdFWnA0TEQ0ODFxeXNsCk5VUzAycElDSVFDMml5NmRLUzlTZmNYdStNYVcxVSt1QWV5WGRTZWhnWTU4QXJYTHI5aDUzUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K",
        "client-key-data": "LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JR0hBZ0VBTUJNR0J5cUdTTTQ5QWdFR0NDcUdTTTQ5QXdFSEJHMHdhd0lCQVFRZ2dsTm9MbVhxN3c0OTduTEEKVFg3TDQyQ283cDVHSVNyOFgvek1yVldTYUo2aFJBTkNBQVRqVUh3RnNmaHlWOWxQYk5TdGphOGh3dEJBU0EvUApBWXIzeUFJNktOWmlLS2pVNDJWNnI5Yk5oM3hHbExiYk5wN1VuKzg2YkRnandlVWtjY0RXanZETwotLS0tLUVORCBQUklWQVRFIEtFWS0tLS0tCg=="
      }
    }
  ],
  "contexts": [
    {
      "name": "default",
      "context": {
        "cluster": "default",
        "user": "default"
      }
    }
  ],
  "current-context": "default"
}`

// Helpers for setting up a ugate test env.

func BasicGate() *ugatesvc.UGate {
	config := ugatesvc.NewConf(".", "./var/lib/dmesh")
	cfg := &ugate.GateCfg{
		BasePort: 12000,
		Domain: "h.webinf.info",
	}
	// Start a Gate. Basic H2 and H2R services enabled.
	ug := ugatesvc.NewGate(&net.Dialer{}, nil, cfg, config)

	return ug
}
