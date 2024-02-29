package main

import (
	"context"
	"net/http"

	"github.com/costinm/meshauth"
	"github.com/costinm/meshauth/util"
	"github.com/costinm/ugate/pkg/ipfs"
)

func main() {
	cfg := &meshauth.MeshCfg{}
	util.MainStart("ugate", cfg)

	auth, err := meshauth.FromEnv(cfg)
	if err != nil {
		panic(err)
	}

	if auth.Cert == nil {
		auth.InitSelfSigned("")
	}

	ipfsdisc, err := ipfs.NewDHT(context.Background(), auth, 11014)
	if err != nil {
		panic(err)
	}

	http.ListenAndServe(":11015", ipfsdisc)
}
