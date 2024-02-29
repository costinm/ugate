package ugated

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/costinm/meshauth"
	sshd "github.com/costinm/ssh-mesh"
	"github.com/costinm/ssh-mesh/nio"
	"github.com/costinm/ssh-mesh/nio/syscall"
	"github.com/costinm/ssh-mesh/sshdebug"
	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/dns"
	echo "github.com/costinm/ugate/pkg/echo"
	"github.com/costinm/ugate/pkg/quic"
	"github.com/costinm/ugate/pkg/sni"
	"github.com/costinm/ugate/pkg/udp"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/costinm/ugate/pkg/http_proxy"
)


const (
	// Offsets from BasePort for the default ports.

	PORT_IPTABLES    = 1
	PORT_IPTABLES_IN = 6
	PORT_SOCKS       = 5
	PORT_DNS         = 3
	// SNI and HTTP could share the same port - would also
	// reduce missconfig risks
	PORT_HTTP_PROXY = 2

	// TLS/SNI, HTTPS (no client certs)
	PORT_HTTPS = 4

	// H2, HTTPS, H2R
	PORT_BTS = 7

	// H2C
	PORT_BTSC = 8

	PORT_MQTT = 9
)


// Init will configure the core modules.
// Other files in this package may intialize optional modules
func Init(ug *ugate.UGate) {
	// sni is the istio-style e-w gateway, blindly forwarding.
	ug.ListenerProto["sni"] = func(gate *ugate.UGate, ll *meshauth.PortListener) error {
		sh := &sni.SNIHandler{
			UGate: ug,
		}
		nio.ListenAndServe(ll.Address, func(conn net.Conn) {
			sh.HandleConn(conn)
		})
		return nil
	}

	// TLS and https ports - can be 443 or a special port.
	ug.ListenerProto["tls"] = func(gate *ugate.UGate, ll *meshauth.PortListener) error {
		nio.ListenAndServe(ll.Address, func(conn net.Conn) {
			s := nio.NewBufferReader(conn)
			defer conn.Close()
			defer s.Buffer.Recycle()

			cn, sni, err := nio.SniffClientHello(s)
			if err != nil {
				return
			}
			if cn == nil {
				// First bytes are not TLS
				return
			}

			log.Println("Got SNI", sni)

			// TODO: match against hostnames to get the cert (if any), else look for TLS forwarding rules.

		})
		return nil
	}

	// hbone is a HTTPS listener exposing forwarding and other mesh functions, typically on the main port.
	//
	ug.ListenerProto["hbone"] = func(gate *ugate.UGate, ll *meshauth.PortListener) (err error) {
		ll.NetListener, err = nio.ListenAndServe(ll.Address, func(conn net.Conn) {
			if ug.TCPUserTimeout != 0 {
				syscall.SetTCPUserTimeout(conn, ug.TCPUserTimeout)
			}

			conf := ug.Auth.GenerateTLSConfigServer(true)
			defer conn.Close()
			tlsConn := tls.Server(conn, conf)
			ctx, cf := context.WithTimeout(context.Background(), 3 * time.Second)
			defer cf()

			err := tlsConn.HandshakeContext(ctx)
			if err != nil {
				return
			}

			alpn := tlsConn.ConnectionState().NegotiatedProtocol
			if alpn != "h2" {
				log.Println("Invalid alpn")
			}
			h2s := &http2.Server{

			}

			h2s.ServeConn(conn, &http2.ServeConnOpts{
				Handler: ug.H2Handler,
				Context: context.Background(),
				//Context: // can be used to cancel, pass meta.
				// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
			})
		})
		return
	}

	// Core functionalty - http  local forwarding.
	ug.ListenerProto["http_proxy"] = func(gate *ugate.UGate, l *meshauth.PortListener) error {
		hproxy := http_proxy.NewHTTPProxy(ug)
		hproxy.HttpProxyCapture(l.Address)
		return nil
	}


	// TODO: virtual listener - not listening but capable of handling tunneled connections.

	ug.ListenerProto["http"] = func(gate *ugate.UGate, ll *meshauth.PortListener) error {
		// TODO: separate mux for each listener, based on routes

		// TODO: populate options based on listener options
		h2s := &http2.Server{
			//MaxHandlers:                  0,
			//MaxConcurrentStreams:         0,
			//MaxDecoderHeaderTableSize:    0,
			//MaxEncoderHeaderTableSize:    0,
			//MaxReadFrameSize:             0,
			//PermitProhibitedCipherSuites: false,
			//IdleTimeout:                  0,
			//MaxUploadBufferPerConnection: 0,
			//MaxUploadBufferPerStream:     0,
			//NewWriteScheduler:            nil,
			//CountError:                   nil,
		}

		// implements the H2C protocol - detects requests with PRI and proto HTTP/2.0 and Upgrade - and calls
		// ServeConn.
		//
		// Otherwise, HTTP/1.1 is used.
		h2ch := h2c.NewHandler(ug.Mux, h2s)

		//h2s.ServeConn(nil, &http2.ServeConnOpts{
		//	Context:          nil,
		//	BaseConfig:       nil,
		//	Handler:          nil,
		//	UpgradeRequest:   nil,
		//	Settings:         nil,
		//	SawClientPreface: false,
		//})

		// TODO: add 	if hb.TCPUserTimeout != 0 {
		//		// only for TCPConn - if this is used for tls no effect
		//		syscall.SetTCPUserTimeout(conn, hb.TCPUserTimeout)
		//	}

		return http.ListenAndServe(ll.Address, h2ch)
	}

	ug.ListenerProto["hbonec"] = ug.ListenerProto["http"]

	ug.ListenerProto["tproxy"] = func(gate *ugate.UGate, l *meshauth.PortListener) error {
		ll, err := nio.IptablesCapture(l.Address, func(c net.Conn, destA, la *net.TCPAddr) {
			//ctx := context.Background()
			//t0 := time.Now()
			//dest := destA.String()

			//nc, err := ug.Dial("tcp", dest)
			//if err != nil {
			//	slog.Info("tproxy dial error", "dest", dest, "err", err)
			//	return
			//}
			//
			//util.Proxy(c, nc, nc, dest)
			c.Write([]byte("hi"))
			c.Close()
			return
		})
		l.NetListener = ll
		return err
	}

	ug.ListenerProto["socks"] = func(gate *ugate.UGate, l *meshauth.PortListener) error {
		ll, err := nio.Sock5Capture(l.Address, func(s *nio.Socks, c net.Conn) {
			//t0 := time.Now()
			dest := s.Dest
			if dest == "" {
				dest = s.DestAddr.String()
			}
			nc, err := ug.Dial("tcp", s.Dest)
			if err != nil {
				s.PostDialHandler(nil, err)
				return
			}
			s.PostDialHandler(nc.LocalAddr(), nil)

			nio.Proxy(nc, c, c, s.Dest)
			return

			//pp, err := sshTransport.Proxy(ctx, dest, c)
			//if err != nil {
			//	slog.Info("SocksDialError", "err", err, "dest", dest)
			//}
			//
			//go func() {
			//	pp.ProxyTo(c)
			//	slog.Info("socks",
			//		"to", dest,
			//		"dur", time.Since(t0),
			//		//"dial", pp.sch.RemoteAddr(),
			//		"in", pp.InBytes,
			//		"out", pp.OutBytes,
			//		"ierr", pp.InErr,
			//		"oerr", pp.OutErr)
			//}()
			//
		})
		l.NetListener = ll
		return err

	}

	ug.ListenerProto["udp"] = func(gate *ugate.UGate, l *meshauth.PortListener) error {
		// Core functionalty - http  local forwarding.
		uh := udp.New(ug, l)
		uh.Listener(l)
		log.Println("uGate: listen UDP ", l.Address, l.ForwardTo)
		return nil
	}

	ug.ListenerProto["admin"] = func(gate *ugate.UGate, ll *meshauth.PortListener) error {
		go func() {
			err := http.ListenAndServe(":"+strconv.Itoa(int(ll.Port)), http.DefaultServeMux)
			if err != nil {
				log.Fatal(err)
			}
		}()
		return nil
	}

	ug.ListenerProto["tproxy_udp"] = func(gate *ugate.UGate, l *meshauth.PortListener) error {
		ul := udp.New(ug, l)
		utp, err := udp.StartUDPTProxyListener6(int(l.Port))
		if utp != nil {
			go udp.UDPAccept(utp, ul.HandleUdp)
		}

		return err
	}

	ug.ListenerProto["ssh"] = func(gate *ugate.UGate, ll *meshauth.PortListener) error {
		gate.SSHConfig.Address = ll.Address
		sshTransport, err := sshd.NewSSHMesh(&gate.SSHConfig)
		if err != nil {
			return err
		}

		// Start internal SSHD/SFTP, only admin can connect.
		// Better option is to install dropbear and start real sshd.
		// Will probably remove this - useful for static binary
		sshTransport.ChannelHandlers["session"] = sshdebug.SessionHandler
		//st.ChannelHandlers["session"] = sshd.SessionHandler

		sshTransport.Forward = func(ctx context.Context, host string, closer io.ReadWriteCloser) {
			str := nio.GetStream(closer, closer)
			defer ug.OnStreamDone(str)
			ug.OnStream(str)

			str.Dest = host
			//str.Direction = ugatesvc.StreamTypeOut
			// TODO: add sniffing on the outbound

			nc, err := ug.DialContext(ctx, "tcp", str.Dest)
			if err != nil {
				return
			}

			nio.Proxy(nc, str, str, str.Dest)
			//defer nc.Close()
			//str.ProxyTo(nc)
		}

		sshTransport.StayConnected(context.Background())

		go sshTransport.Start()
		return nil
	}

	ug.ListenerProto["tcp"] = func(gate *ugate.UGate, ll *meshauth.PortListener) error {
		if ll.ForwardTo == "" {
			log.Println("No route", ll.Address)
		}
		l, err := net.Listen("tcp", ll.Address)
		if err != nil {
			return err
		}
		go func() {
			for {
				a, err := l.Accept()
				if err != nil {
					log.Println("Error accepting", err)
					return
				}
				go func() {
					a := a

					ctx := context.Background()

					nc, err := ug.DialContext(ctx, "", ll.ForwardTo)
					if err != nil {
						log.Println("RoundTripStart error", ll.ForwardTo, err)
						a.Close()
						return
					}
					err = nio.Proxy(nc, a, a, ll.ForwardTo)
					if err != nil {
						log.Println("FWD", ll.Address, a.RemoteAddr(), err)
					} else {
						log.Println("FWD", ll.Address, a.RemoteAddr())
					}
				}()
			}
		}()
		return nil
	}

	dnss, _ := dns.NewDmDns(ug.BasePort + 3) // 5223

	net.DefaultResolver.PreferGo = true
	net.DefaultResolver.Dial = dns.DNSDialer(ug.BasePort + 3)

	ug.DNS = dnss
	ug.Mux.Handle("/dns/", dnss)

	ug.StartFunctions = append(ug.StartFunctions, func(ug *ugate.UGate) {
		go dnss.Serve()
	})


	ug.ListenerProto["echo"] = echo.EchoPortHandler

	quic.InitQuic(ug)

}

