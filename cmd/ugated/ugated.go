package main

import (
	"context"
	"fmt"
	"github.com/costinm/ugate/pkg/ipfs"
	"log"
	_ "net/http/pprof"
	"os"

	"github.com/costinm/meshauth"
	meshauth_util "github.com/costinm/meshauth/util"
	sshd "github.com/costinm/ssh-mesh"
	"github.com/costinm/ugate"
	"github.com/costinm/ugate/ugated"
	"sigs.k8s.io/yaml"

	_ "github.com/costinm/ugate/pkg/ext/gvisor"
	//_ "github.com/costinm/ugate/pkg/ext/ipfs"
	_ "github.com/costinm/ugate/pkg/ext/lwip"

	"golang.org/x/exp/slog"
)

// Multiprotocol mesh gateway
//
// - iptables capture
// - option to use mTLS - if the network is secure ( ipsec or equivalent ) no encryption
// - detect TLS and pass it through
// - inbound: extract metadata
// - DNS and DNS capture (if root)
// - control plane using webpush messaging
// - webRTC and H2 for mesh communication
// - convert from H2/H1, based on local port config.
// - SOCKS and PROXY
//
// This does not include TUN+lwIP, which is now only used with AndroidVPN in
// JNI mode (without many of the extras).
func main() {
	ctx := context.Background()
	// First step in a 'main' app is to bootstrap
	// - identity - certs, JWTs, other sources - maybe self-signed if first boot
	// - control plane and core config/discovery services - MDS, k8s, XDS
	// - env variables and local files to locate the CP and overrides/defaults

	// SetCert configs from the current dir and var/lib/dmesh, or env variables
	// Writes to current dir.
	// TODO: other config sources, based on env

	// Use the detected config storage to load mesh settings
	meshCfg := &ugate.MeshSettings{}
	basecfg := meshauth_util.FindConfig("ugate", ".yaml")
	if basecfg != nil {
		err := yaml.Unmarshal(basecfg, meshCfg)
		if err != nil {
			panic(err)
		}
	}

	// ===== SSH protocol config =================
	//
	sshCfg := &meshCfg.SSHConfig

	// SetCert keys, certs, configs from ~/.ssh/ - unless ssh was configured in the base config
	if sshCfg.Private == "" {
		sshd.EnvSSH(sshCfg)
	}

	// From sshm.go
	authn := meshauth.NewAuthn(&sshCfg.AuthnConfig)

	if len(sshCfg.AuthnConfig.Issuers) > 0 {
		err := authn.FetchAllKeys(ctx, sshCfg.AuthnConfig.Issuers)
		if err != nil {
			log.Println("Issuers", err)
		}
		sshCfg.TokenChecker = authn.CheckJwtMap
	}

	// ========== Mesh Auth ======================
	// TODO: use SSH config to bootstrap

	// init defaults for the rest
	if meshCfg.BasePort == 0 {
		meshCfg.BasePort = 14000
	}

	// Initialize the authentication and core config, using env
	// variables, detection and local files.
	meshAuth, _ := meshauth.FromEnv(&meshCfg.MeshCfg)
	if meshAuth.Cert == nil {
		// If no Cert was found - generate a self-signed for bootstrap.
		// Additional mesh certs can be added later, if a control plane is found.
		meshAuth.InitSelfSigned("")
		// TODO: only if mesh control plane not set.
		meshAuth.SaveCerts(".")
	}

	// ========= UGateHandlers routing and extensions ===========

	ug := ugate.New(meshAuth, meshCfg)

	if ug.Listeners["hbonec"] == nil {
		// Set if running in a knative env, or if an Envoy/ambient runs as a sidecar to handle
		// TLS, QUIC, H2. In this mode only standard H2/MASQUE are supported, with
		// reverse connections over POST or websocket.
		knativePort := os.Getenv("PORT")
		haddr := ""
		if knativePort != "" {
			haddr = ":" + knativePort
		} else {
			haddr = fmt.Sprintf("0.0.0.0:%d", meshCfg.BasePort)
		}
		ug.Listeners["hbonec"] =
			&meshauth.PortListener{
			Address: haddr,
			Protocol: "hbonec"}
	}

	if ug.Listeners["ssh"] == nil {
		ug.Listeners["ssh"] = &meshauth.PortListener{
			Address: fmt.Sprintf(":%d", meshCfg.BasePort + 22),
			Protocol: "ssh"}
	}
	if ug.Listeners["socks"] == nil {
		ug.Listeners["socks"] = &meshauth.PortListener{
			Address: fmt.Sprintf("127.0.0.1:%d", meshCfg.BasePort + 8),
			Protocol: "socks"}
	}
	if ug.Listeners["tproxy"] == nil {
		ug.Listeners["tproxy"] = &meshauth.PortListener{
			Address: fmt.Sprintf(":%d", meshCfg.BasePort + 6),
			Protocol: "tproxy"}
	}
	if ug.Listeners["quic"] == nil {
		ug.Listeners["quic"] = &meshauth.PortListener{
			Address: fmt.Sprintf(":%d", meshCfg.BasePort + 9),
			Protocol: "quic"}
	}

	// Register core protocols.
	ugated.Init(ug)

	ipfs.Init(ug)

	ug.Start()
	// Start a SSH mesh node. This allows other authorized local nodes to jump and a debug
	// interface.

	for _, h := range ug.StartFunctions {
		go h(ug)
	}

	// Log - may be sent to otel.
	slog.Info("ugate/start",
		"meshEnv", ug.Auth.ID,
		"name", ug.Auth.Name,
		"basePort", ug.BasePort,
		"pub", meshauth.PublicKeyBase32SHA(meshauth.PublicKey(ug.Auth.Cert.PrivateKey)),
		"vip", ug.Auth.VIP6)

	meshauth_util.MainEnd()
}
