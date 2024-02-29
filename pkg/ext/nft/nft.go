package nft

import (
	"github.com/google/nftables"
	"github.com/greenpau/cni-plugins/pkg/utils"
)

/*

nft list ruleset

*/

func Nft() {
	nftables.New()

	// They use /proc/%d/task/%d/ns/net to get the net NS.

	// Named networks:  /var/run/netns/NAME

	utils.CreateTable("4", "mesh")
	utils.CreateTable("6", "mesh")
}
