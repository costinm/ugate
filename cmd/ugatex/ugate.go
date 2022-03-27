package main

import (
	"log"
	_ "net/http/pprof"

	_ "github.com/costinm/ugate/ext/bootstrap"
	_ "github.com/costinm/ugate/ext/bootstrapx"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

// Full function gate - includes all extensions.
func main() {
	// Load configs from the current dir and var/lib/dmesh, or env variables
	// Writes to current dir.
	config := ugatesvc.NewConf("./", "./var/lib/dmesh")
	_, err := ugatesvc.Run(config, nil)
	if err != nil {
		log.Fatal(err)
	}
	select {}
}
