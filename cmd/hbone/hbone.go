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
	// WIP:
	//	port = flag.Int("l", 0, "local port")
	//debugPort = flag.Int("d", 0, "debug/status port")
)



var hc *http.Client

// Create a HBONE tunnel to a given URL.
//
// Current client is authenticated for HBONE using local credentials, or a kube.json file.
// If no kube.json is found, one will be generated.
//
// Example:
// ssh -v -o ProxyCommand='hbone https://c1.webinf.info:443/dm/PZ5LWHIYFLSUZB7VHNAMGJICH7YVRU2CNFRT4TXFFQSXEITCJUCQ:22'  root@PZ5LWHIYFLSUZB7VHNAMGJICH7YVRU2CNFRT4TXFFQSXEITCJUCQ
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

	if len(flag.Args()) == 0 {
		log.Fatal("Expecting URL")
	}
	url := flag.Arg(0)
	err := Netcat(ug, url)
	if err != nil {
		log.Fatal(err)
	}
}

// Netcat copies stdin/stdout to a HBONE stream.
func Netcat(ug *ugatesvc.UGate, s string) error {
	i, o := io.Pipe()
	r, _ := http.NewRequest("POST", s, i)
	res, err := ug.RoundTrip(r)
	if err != nil {
		return err
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
			return err
		}
		nc.Write(b1[0:n])
	}
	return nil
}

