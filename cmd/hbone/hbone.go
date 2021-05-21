package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/auth"
	"github.com/costinm/ugate/pkg/ugatesvc"
)


var (
	verbose = flag.Bool("v", false, "Verbose messages")
)

var hc *http.Client

// Create a HBONE tunnel to a given URL.
//
// Current client is authenticated using local credentials, or a kube.json file. If no kube.json is found, one
// will be generated.
//
//
//
// Example:
// ssh -v -o ProxyCommand='wp -nc https://c1.webinf.info:443/dm/PZ5LWHIYFLSUZB7VHNAMGJICH7YVRU2CNFRT4TXFFQSXEITCJUCQ:22'  root@PZ5LWHIYFLSUZB7VHNAMGJICH7YVRU2CNFRT4TXFFQSXEITCJUCQ
//
// Bug: %h:%p doesn't work, ssh uses lower case and confuses the map.
func main() {
	flag.Parse()

	config := ugatesvc.NewConf("./", "./var/lib/dmesh/")
	authz := auth.NewAuth(config, "", "")

	ug := ugatesvc.New(config, authz, nil)

	hc = &http.Client{
		Transport: ug,
	}


	url := ""
	if len(os.Args) > 1 {
		url = os.Args[1]
	}
	if url == "" {
		log.Fatal("Expecting URL")
	}

	Netcat(ug, url)
}

func Netcat(ug *ugatesvc.UGate, s string) {
	i, o := io.Pipe()
	r, _ := http.NewRequest("POST", s, i)
	res, err := ug.RoundTrip(r)
	if err != nil {
		log.Fatal(err)
	}
	nc := ugate.NewStreamRequestOut(r, o, res, nil)
	go func() {
		b1 := make([]byte, 1024)
		for {
			n, err := nc.Read(b1)
			if err != nil {
				log.Fatal("Tun read err", err)
			}
			os.Stdout.Write(b1[0:n])
		}
	}()
	b1 := make([]byte, 1024)
	for {
		n, err := os.Stdin.Read(b1)
		if err != nil {
			log.Fatal("Stding read err", err)
		}
		nc.Write(b1[0:n])
	}
}

