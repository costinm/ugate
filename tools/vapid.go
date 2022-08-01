package main

import (
	"fmt"
	"os"

	"github.com/costinm/ugate/auth"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

// Generate a self-signed VAPID token
// First param is the audience
func main() {
	aud := "example.com"
	if len(os.Args) > 0 {
		aud = os.Args[0]
	}

	config := ugatesvc.NewConf("./", "./var/lib/dmesh/")
	authz := auth.NewAuth(config, "", "")
	fmt.Println(authz.VAPIDToken(aud))
}
