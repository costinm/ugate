//go:build ipfs
// +build ipfs
package cmd

import (
	"github.com/costinm/meshauth"
	"github.com/costinm/ugate/pkg/ipfs"
)

func init() {
	meshauth.Register("ipfs", ipfs.NewLibP2P)
	meshauth.Register("ipfsdht", ipfs.NewDht)
}
