package main

import (
	"fmt"
	"os"

	"github.com/costinm/meshauth"
)

// Generate a self-signed VAPID token
// First param is the audience
func main() {
	aud := "example.com"
	if len(os.Args) > 0 {
		aud = os.Args[0]
	}

	authz := meshauth.NewAuth("", "")
	fmt.Println(authz.VAPIDToken(aud))
}
