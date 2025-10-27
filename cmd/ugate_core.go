package cmd

import (
	"net"
	"net/http"
	"strconv"

	"github.com/costinm/meshauth"
	"github.com/costinm/meshauth/pkg/certs"
	"github.com/costinm/meshauth/pkg/ugcp"
	"github.com/costinm/meshauth/pkg/uk8s"
	"github.com/costinm/ssh-mesh/nio"
	"github.com/costinm/ugate/pkg/h2r"
	"github.com/costinm/ugate/pkg/local_discovery"
	"github.com/costinm/ugate/pkg/smtpd"

	"github.com/costinm/ugate/appinit"

	"github.com/costinm/meshauth/pkg/tokens"

	"github.com/costinm/ssh-mesh/pkg/h2"
	ssh_mesh "github.com/costinm/ssh-mesh/pkg/ssh"
	"github.com/costinm/ugate/pkg/dns"
	"github.com/costinm/ugate/pkg/echo"
	"github.com/costinm/ugate/pkg/http_proxy"
	"github.com/costinm/ugate/pkg/udp"
	msgs "github.com/costinm/ugate/pkg/webpush"
)

// TODO: HA_PROXY or callback support for getting peer info.
// TODO: socks support for outbound calls

func init() {
	Register()
}

// Register registers a number of common/core modules to appinit.
func Register() {

	// Meshauth package
	appinit.RegisterN("ssh", ssh_mesh.New)

	appinit.RegisterN("certs", certs.NewCerts)

	appinit.RegisterN("trust", certs.NewTrust)

	appinit.RegisterN("mux", func() *http.ServeMux { return http.NewServeMux() })

	appinit.RegisterN("k8s", uk8s.New) // uk8s.K8Service{})

	// Networking - dialer, listeners
	appinit.RegisterN("mesh", meshauth.New)

	appinit.RegisterN("mdsd", ugcp.NewServer)
	appinit.RegisterN("mds", ugcp.New)

	// OIDC authentication config - if any issuer is found, keys are fetched.
	appinit.RegisterT("authn", &tokens.Authn{})

	appinit.RegisterN("smtp", smtpd.New)

	appinit.RegisterT("proxyv1", &h2.Proxy1{})

	// Old modules using Mesh directly.

	// CBOR:
	// indigo (atproto) - github.com/whyrusleeping/cbor and cbor-gen
	//     8 year old 1-file cbor - same author as indigo
	//
	// k8s: github.com/fxamacker/cbor/v2
	//  - MarshalToBuffer
	//  - UnmarshalFirst - sequences
	//
	// ugorji/go

	appinit.RegisterN("msgmux", msgs.NewMux)

	appinit.RegisterN("dns", dns.New)

	//meshauth.Register("h2", func(l *meshauth.Module) error {
	//	ma := l.Mesh
	//	l.ConnHandler = func(conn net.Conn) {
	//		if ma.TCPUserTimeout != 0 {
	//			syscall.SetTCPUserTimeout(conn, ma.TCPUserTimeout)
	//		}
	//
	//
	//		conf := ma.Cert.GenerateTLSConfigServer(nil)
	//		defer conn.Close()
	//		tlsConn := tls.Server(conn, conf)
	//		ctx, cf := context.WithTimeout(context.Background(), 3*time.Second)
	//		defer cf()
	//
	//		err := tlsConn.HandshakeContext(ctx)
	//		if err != nil {
	//			return
	//		}
	//
	//		alpn := tlsConn.ConnectionState().NegotiatedProtocol
	//		if alpn != "h2" {
	//			log.Println("Invalid alpn")
	//		}
	//		h2s := &http2.Server{}
	//
	//		//  h2c.NewHandler(maCfg.MainMux, &http2.Server{}),
	//
	//		h2s.ServeConn(conn, &http2.ServeConnOpts{
	//			Handler: mhttp.NewAuthHandler(ma, ma.Mux),
	//			Context: context.Background(),
	//			//Context: // can be used to cancel, pass meta.
	//			// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
	//		})
	//	}
	//	return nil
	//})

	appinit.RegisterT("http_proxy", &http_proxy.HttpProxy{})

	appinit.RegisterN("udp", udp.New)
	appinit.RegisterT("udp_tproxy", &udp.UDPTproxy{})
	appinit.RegisterT("tcp", &nio.Listener{})

	appinit.RegisterT("extauthz", &tokens.Authz{})

	// TODO: expose http.DefaultServerMux with admin pass (for debug, etc)

	appinit.RegisterT("h2r", &h2r.H2R{})

	appinit.RegisterT("echo", &echo.EchoHandler{})

}

func StartDiscovery() {
	disc := &local_discovery.LLDiscovery{}
	disc.Start()
}

func GetPort(a string, dp int32) int32 {
	if a == "" {
		return dp
	}
	_, p, err := net.SplitHostPort(a)
	if err != nil {
		return dp
	}
	pp, err := strconv.Atoi(p)
	if err != nil {
		return dp
	}
	return int32(pp)
}
