package main

import (
	"context"
	"log/slog"
	_ "net/http/pprof"
	"os"

	"github.com/costinm/ugate/appinit"

	"github.com/costinm/ssh-mesh/pkg/h2"
	"github.com/costinm/ssh-mesh/pkg/ssh"

	_ "github.com/costinm/ugate/cmd"

	"github.com/go-json-experiment/json"
	"github.com/goccy/go-yaml"
	//_ "github.com/costinm/ugate/pkg/ext/ipfs"
	//_ "github.com/costinm/ugate/pkg/ext/lwip"
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

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{})))

	// Register all modules in ugate and main deps
	appinit.Yaml2Json = Yaml2JSON
	// Use fancy json and yaml parsers (to test them out)
	appinit.Unmarshallers["json"] = func(bytes []byte, a any) error {
		return json.Unmarshal(bytes, a)
	}

	// Allow creation of more and H2 SSH servers
	appinit.RegisterN("sshd", ssh.New)
	appinit.RegisterT("h2cd", &h2.H2{})
	// A H2C RoundTripper.
	appinit.RegisterT("h2c", &h2.H2C{})

	// End registration for available modules

	// Set the config repository
	b := os.DirFS(".")
	cs := appinit.AppResourceStore()

	err := cs.Load(ctx, b, ".")
	if err != nil {
		slog.Error("RecordError", "err", err)
		panic(err)
	}

	// Other manually registered objects.
	// s := sshcmd.NewSSHM()
	// s.SSH.FromEnv()
	// cs.Set("/ssh/default", s.SSH)
	// cs.Set("/h2/default", s.H2)
	// err = s.Provision(ctx)
	// if err != nil {
	// 	log.Error("Error provisioning", err, cs)
	// 	panic(err)
	// }

	// Start all 'services' in the config
	err = cs.Start()
	if err != nil {
		panic(err)
	}

	// s is registered to the app store - so will start along with the rest.
	// s.Start(ctx)
	appinit.WaitEnd()
}

func Yaml2JSON(bb []byte) ([]byte, error) {
	raw := map[string]any{}
	err := yaml.Unmarshal(bb, &raw)
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}
