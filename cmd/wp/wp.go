// Command line tool to generate VAPID keys and tokens
// The subscription can be provided as JSON, or as separate flags
// The message to be sent must be provided as stdin or 'msg'
// The VAPID key pair should be set as environment variables, not in commaond line.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/auth"
	"github.com/costinm/ugate/pkg/msgs"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/costinm/wpgate/pkg/h2"
	"github.com/costinm/wpgate/pkg/transport/eventstream"
)

var (
	// vapid  = flag.NewFlagSet("vapid", flag.ExitOnError)
	// sub = vapid.String("sub", "", "Optional email or URL identifying the sender")
	// vapid.Parse(os.Args[2:])

	to = flag.String("to", "", "Destination, if not set will print info. A VIP6 or known hostname")

	// Mesh: currently use .ssh/authorized_keys, known_hosts
	// Webpush: file under .ssh/webpush/NAME or TO. Inside
	sub = flag.String("sub", "",
		"Optional email or URL identifying the sender, to look up the subscription")

	aud  = flag.String("aud", "", "Generate a VAPID key with the given domain. Defaults to https://fcm.googleapis.com")
	curl = flag.Bool("curl", false, "Show curl request")

	watch      = flag.Bool("watch", false, "Watch")
	dump      = flag.Bool("dump", false, "Dump id and authz")
	dumpKnown = flag.Bool("k", false, "Dump known hosts and keys")

	sendVerbose = flag.Bool("v", false, "Show request and response body")

	pushService = flag.String("server", "", "Base URL for the dmesh service")
)

var (
	port = flag.Int("p", -1, "Port for http interface. Will run as daemon")

	watchRecurse   = flag.Bool("r", false, "Connect recursively to all nodes")
	verbose = flag.Bool("v", false, "Verbose messages")

	dev = flag.String("d", "", "Single device to watch")

	post  = flag.String("m", "", "Message to send")
	topic = flag.String("t", "", "Message to send")
)

var hc *http.Client



const (
	Subscription = "TO"
)

type Keys struct {
	P256dh string ``
	Auth   string ``
}

type Sub struct {
	Endpoint string ``
}

// Send the message.
func sendMessage(toS string, vapid *auth.Auth, show bool, msg string) {
	//msg, err := ioutil.ReadAll(os.Stdin)
	//if err != nil {
	//	fmt.Println("Failed to read message")
	//	os.Exit(3)
	//}

	destURL := ""
	var destPubK []byte
	var authk []byte

	// browser sub: real webpush
	wpSub := os.Getenv(Subscription)
	if len(wpSub) > 0 {
		to, err := msgs.SubscriptionFromJSON([]byte(wpSub))
		if err != nil {
			fmt.Println("Invalid endpoint "+flag.Arg(1), err)
			os.Exit(3)
		}
		destURL = to.Endpoint
		destPubK = to.Key
		authk = to.Auth
	} else {
		subs := ugate.ConfStr(vapid.Config, "sub_"+toS+".json", "")
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
		} else {
			//// DMesh nodes - we only have the public key, auth is not sent !
			//az := vapid.Known[toS]
			//if az == nil {
			//	az = vapid.Authz[toS]
			//}
			//if az == nil {
			log.Println("Not found ", toS)
			//	return
			//}
			//destPubK = az.Public
			//vip := auth.Pub2VIP(destPubK).String()
			//destURL = "https://[" + vip + "]:5228/push/"
			//authk = []byte{1}
		}
	}
	var hc *http.Client

	if *pushService != "" {
		destURL = *pushService + "/push/"
		hc = h2.InsecureHttp()
	} else {
		hc = h2.NewSocksHttpClient("127.0.0.1:5224")
	}

	ec := auth.NewContextSend(destPubK, authk)
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

	//hc := h2.ProxyHttp("127.0.0.1:5203")
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

	} else if *sendVerbose {
		dmpReq, _ := httputil.DumpRequest(req, true)
		fmt.Printf(string(dmpReq))
		dmp, _ := httputil.DumpResponse(res, true)
		fmt.Printf(string(dmp))
	}
}

func main() {
	flag.Parse()

	// Using the SSH directory for keys and target.
	cfgDir := os.Getenv("HOME") + "/.ssh/"
	// File-based config
	config := ugatesvc.NewConf(cfgDir, "./var/lib/dmesh/")
	hn, _ := os.Hostname()

	authz := auth.NewAuth(config, hn, "m.webinf.info")

	if *watch {
		watchNodes()
		return
	}
	//if *dump {
	//	authz.Dump()
	//	return
	//}
	//if *dumpKnown {
	//	authz.DumpKnown()
	//	return
	//}

	sendMessage(*to, authz, *curl, flag.Args()[0])
}

// Known nodes and watchers
var watchers = map[uint64]*watcher{}

type watcher struct {
	Node *ugate.DMNode
	Path []string

	dirty bool

	base string
	ip6  *net.IPAddr

	idhex    string
	New      bool
	watching bool
}


func watchNodes() {
	var hc *http.Client
	if *pushService != "" {
		hc = h2.InsecureHttp()
	} else {
		hc = h2.NewSocksHttpClient("127.0.0.1:5224")
	}
	hc.Timeout = 1 * time.Hour

	mux := msgs.DefaultMux

	// Recursive list of all devices in the mesh.
	for {
		listOnce()
		if !*watch {
			return
		} else {
			//if *dev == "*" || *dev == "" {
			//	go msgs.DefaultMux.MonitorNode(hc, nil, nil)
			//}
			for _, w := range watchers {
				//if w.idhex == *dev || *dev == "*" {
				if !w.watching {
					w.watching = true
					go func() {
						log.Println("Watch: ", w.ip6)

						eventstream.MonitorNode(mux, hc, w.ip6)
						w.watching = false
						log.Println("Watch close: ", w.ip6)
					}()
				}

				//}
			}
		}
		time.Sleep(10 * time.Second)
		//select {}
	}

}

// Get neighbors from a node. Side effect: updates the watchers table.
func neighbors(url string, path []string) map[uint64]*ugate.DMNode {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Print("HTTP_ERROR1", url, err)
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	req = req.WithContext(ctx)

	hcc := hc
	if strings.HasPrefix(url, "http://127") {
		hcc = http.DefaultClient
	}
	res, err := hcc.Do(req)
	if err != nil || res.StatusCode != 200 {
		log.Println("HTTP_ERROR2", url, err)
		return nil
	}

	cnodes, _ := ioutil.ReadAll(res.Body)
	cnodemap := map[uint64]*ugate.DMNode{}
	err = json.Unmarshal(cnodes, &cnodemap)
	if err != nil {
		log.Println("HTTP_ERROR_RES", url, err, string(cnodes))
		return nil
	}

	peers := []string{}

	newdemap := map[uint64]*watcher{}
	oldmap := map[uint64]*watcher{}
	for k, v := range cnodemap {
		w, f := watchers[k]
		if !f {
			ip6 := toIP6(k)
			w = &watcher{Node: v, dirty: true, ip6: ip6,
				idhex: fmt.Sprintf("%x", k),
				Path:  path}
			w.New = true
			watchers[k] = w
			newdemap[k] = w
		} else {
			oldmap[k] = w
		}
		peers = append(peers, w.idhex)
	}
	if *verbose {
		log.Println("GET", url, len(cnodemap), peers)
	}
	return cnodemap
}

// Scan all nodes for neighbors. Watch them if not watched already.
func listOnce() {
	for _, v := range watchers {
		v.dirty = true
		v.New = false
	}

	// Will update watchers
	neighbors("http://127.0.0.1:5227/dmesh/ip6", nil)

	more := true

	for more {
		more = false
		for _, v := range watchers {
			if !v.dirty {
				continue
			}
			more = true
			p := "/"
			for _, pp := range v.Path {
				p = p + pp + "/"
			}
			p = p + v.idhex + "/"

			//neighbors("http://127.0.0.1:5227/dm"+p+"/c/dmesh/ip6", append(v.Path, v.idhex))
			neighbors("http://"+net.JoinHostPort(v.ip6.String(), "5227")+"/dmesh/ip6", append(v.Path, v.idhex))
			v.dirty = false
		}
	}

	for _, v := range watchers {
		if v.New {
			log.Println("Node", v.idhex, v.Path, v.Node.NodeAnnounce.UA, v.Node.GWs())
		}
	}
}

func stdinClient(mux *msgs.Mux) {
	stdin := &msgs.MsgConnection{
		Name:                "",
		SubscriptionsToSend: []string{"*"},
		SendMessageToRemote: func(ev *msgs.Message) error {
			ba := ev.MarshalJSON()
			os.Stdout.Write(ba)
			os.Stdout.Write([]byte{'\n'})
			return nil
		},
	}
	mux.AddConnection("stdin", stdin)

	go func() {
		br := bufio.NewReader(os.Stdin)
		for {
			line, _, err := br.ReadLine()
			if err != nil {
				break
			}
			if len(line) > 0 && line[0] == '{' {
				ev := msgs.ParseJSON(line)
				mux.SendMessage(ev)
			}
		}
	}()


}

func toIP6(k uint64) *net.IPAddr {
	ip6, _ := net.ResolveIPAddr("ip6", "fd00::")
	binary.BigEndian.PutUint64(ip6.IP[8:], k)
	return ip6
}

func (w *watcher) monitor(addr *net.IPAddr) {

	req, err := http.NewRequest("GET", "http://["+addr.String()+"]:5227/debug/events", nil)
	if err != nil {
		return
	}
	res, err := hc.Do(req)
	if err != nil {
		println(addr, "HTTP_ERROR", err)
		return
	}
	rd := bufio.NewReader(res.Body)

	for {
		l, _, err := rd.ReadLine()
		if err != nil {
			if err.Error() != "EOF" {
				log.Println(addr, "READ ERR", err)
			}
			break
		}
		ls := string(l)
		if ls == "" || ls == "event: message" {
			continue
		}

		log.Println(addr, ls)
	}

}
