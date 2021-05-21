package main

import (
	"fmt"

	"github.com/costinm/ugate/pkg/auth"
	"github.com/costinm/ugate/pkg/ugatesvc"
)

// Test program to generate a kube config, as used in ugate.
//
func main() {

	// In memory
	config := ugatesvc.NewConf()
	_ = auth.NewAuth(config, "", "")
	gen, _ := config.Get("kube.json")
	fmt.Println(string(gen))
	return

}
