package main

import (
	"net"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/msgs"
	ug "github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/costinm/wpgate/ipfs"
)

// Start a uGate + IPFS
// Env:
// IPFS_ROOT - connect to this IPFS upstream server. If unset, use bootstrap peers
// DHT - if empty, will use a DHT. Else - in memory Peerstore
func main() {
	// Load configs from the current dir and var/lib/dmesh, or env variables
	// Writes to current dir.
	config := ug.NewConf("./", "./var/lib/dmesh")

	// Start a Gate. Basic H2 and H2R services enabled.
	ug := ug.NewGate(&net.Dialer{}, nil,  &ugate.GateCfg{
		BasePort: 16000,
	}, config)

	msgs.DefaultMux.Auth = ug.Auth

	// Start a basic ipfs libp2p service.
	ipfsg := ipfs.InitIPFS(ug.Auth, ug.Config.BasePort + 100, ug.Mux)


	ug.Mux.Handle("/ipfs/", ipfsg)

	select {

	}
}
