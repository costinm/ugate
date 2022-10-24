package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/costinm/meshauth"
	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/ugatesvc"
	msgs "github.com/costinm/ugate/webpush"
)

var (
	to      = flag.String("to", "", "Destination. A config file or env variable required")
	data    = flag.String("data", "", "Message to send, if empty stdin will be used")
	verbose = flag.Bool("v", false, "Verbose messages")
)

var hc *http.Client

// Send a signed and encrypted message to a node, using Webpush protocol.
//
// This is used for control and events.
func main() {
	flag.Parse()

	config := ugatesvc.NewConf("./", "./var/lib/dmesh/")
	authz, _ := meshauth.FromEnv("", false)

	// Use ug as transport - will route to the mesh nodes.
	ug := ugatesvc.New(config, authz, nil)

	hc = &http.Client{
		Transport: ug,
	}

	sendMessage(config, *to, authz, *verbose, *data)
}

// Send an encrypted message to a node.
func sendMessage(config ugate.ConfStore, toS string, vapid *meshauth.MeshAuth, show bool, msg string) {
	var err error
	if msg == "" {
		msgB, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			fmt.Println("Failed to read message")
			os.Exit(3)
		}
		msg = string(msgB)
	}

	destURL := ""
	var destPubK []byte
	var authk []byte

	// Format expected to be same as the browser sub in webpush
	subs := ugate.ConfStr(config, "sub_"+toS, "")
	if subs != "" {
		to, err := msgs.SubscriptionFromJSON([]byte(subs))
		if err != nil {
			fmt.Println("Invalid endpoint "+flag.Arg(1), err)
			os.Exit(3)
		}
		destURL = to.Endpoint
		destPubK = to.Key
		authk = to.Auth
		if len(authk) == 0 {
			authk = []byte{1}
		}
	}

	ec := meshauth.NewContextSend(destPubK, authk)
	c, _ := ec.Encrypt([]byte(msg))

	ah := vapid.VAPIDToken(destURL)

	if show {
		payload64 := base64.StdEncoding.EncodeToString(c)
		cmd := "echo -n " + payload64 + " | base64 -d > /tmp/$$.bin; curl -XPOST --data-binary @/tmp/$$.bin"
		cmd += " -proxy 127.0.0.1:5224"
		cmd += " -Httl:0"
		cmd += " -H\"authorization:" + ah + "\""
		fmt.Println(cmd + " " + destURL)

		return
	}

	req, _ := http.NewRequest("POST", destURL, bytes.NewBuffer(c))
	req.Header.Add("ttl", "0")
	req.Header.Add("authorization", ah)
	req.Header.Add("Content-Encoding", "aes128gcm")

	res, err := hc.Do(req)

	if res == nil {
		fmt.Println("Failed to send ", err)

	} else if err != nil {
		fmt.Println("Failed to send ", err)

	} else if res.StatusCode != 201 {
		//dmpReq, err := httputil.DumpRequest(req, true)
		//fmt.Printf(string(dmpReq))
		dmp, _ := httputil.DumpResponse(res, true)
		fmt.Println(string(dmp))
		fmt.Println("Failed to send ", err, res.StatusCode)

	} else if *verbose {
		dmpReq, _ := httputil.DumpRequest(req, true)
		fmt.Printf(string(dmpReq))
		dmp, _ := httputil.DumpResponse(res, true)
		fmt.Printf(string(dmp))
	}
}
