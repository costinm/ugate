package main

import (
	"flag"
	"log"
	"net"
	"os"

	"github.com/costinm/ugate/pkg/auth"
	"github.com/costinm/ugate/pkg/ugatesvc"
)


var (
	port = flag.String("l", "", "local port")
	tls = flag.Bool("s", false, "mTLS over hbone")
)


// Create a HBONE tunnel to a given URL.
//
// Current client is authenticated for HBONE using local credentials,
// or a kube.json file. If no certs or kube.json is found, one will be generated.
//
// Example:
// ssh -v -o ProxyCommand='hbone https://c1.webinf.info:443/dm/PZ5LWHIYFLSUZB7VHNAMGJICH7YVRU2CNFRT4TXFFQSXEITCJUCQ:22'  root@PZ5LWHIYFLSUZB7VHNAMGJICH7YVRU2CNFRT4TXFFQSXEITCJUCQ
// ssh -v -o ProxyCommand='hbone https://%h:443/hbone/:22' root@fortio.app.run
//
// Note that SSH is converting %h to lowercase - the ID must be in this form
//
func main() {
	flag.Parse()

	config := ugatesvc.NewConf("./", "./var/lib/dmesh/")
	authz := auth.NewAuth(config, "", "")

	ug := ugatesvc.New(config, authz, nil)

	if len(flag.Args()) == 0 {
		log.Fatal("Expecting URL")
	}
	url := flag.Arg(0)

	if *port != "" {
		l, err := net.Listen("tcp", *port)
		if err != nil {
			panic(err)
		}
		for {
			a, err := l.Accept()
			if err != nil {
				panic(err)
			}
			go func() {
				err := ugatesvc.HboneCat(ug, url, *tls, a, a)
				if err != nil {
					log.Println(err)
				}
			}()
		}
	}

	err := ugatesvc.HboneCat(ug, url, *tls, os.Stdin, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
}


