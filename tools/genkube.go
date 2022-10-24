package main

import (
	"github.com/costinm/meshauth"
)

// Test program to generate a kube config, as used in ugate.
func main() {

	// In memory
	_ = meshauth.NewAuth("", "")
	return

}
