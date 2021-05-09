// Command line tool to generate VAPID keys and tokens
// The subscription can be provided as JSON, or as separate flags
// The message to be sent must be provided as stdin or 'msg'
// The VAPID key pair should be set as environment variables, not in commaond line.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
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
	"github.com/costinm/ugate/pkg/ugatesvc"
	msgs "github.com/costinm/ugate/webpush"
)

var (
	to = flag.String("to", "", "Destination, default to local node")

	addr = flag.String("addr", "127.0.0.1:15007", "address:port for the node")

	// Mesh: currently use .ssh/authorized_keys, known_hosts
	// Webpush: file under .ssh/webpush/NAME or TO. Inside
	hostid = flag.String("host", "",
		"Optional email or URL identifying the sender, to look up the subscription")

	sub = flag.String("sub", "", "Subscribe")

	aud  = flag.String("aud", "", "Generate a VAPID key with the given domain. Defaults to https://fcm.googleapis.com")

	watch      = flag.Bool("watch", false, "Watch")
	recurse      = flag.Bool("r", false, "Crawl and watch all possible nodes")

	pushService = flag.String("server", "", "Base URL for the dmesh service")


	jwt=flag.String("jwt", "", "JWT to decode")
	data=flag.String("data", "", "Message to send, if empty stdin will be used")

	netcat = flag.String("nc", "", "Netcat")

	kubeconfig = flag.Bool("kubeconfig", false, "Generate a kube config with key/cert")

)

var (
	verbose = flag.Bool("v", false, "Verbose messages")
)

var hc *http.Client

const (
	Subscription = "TO"
)

func main() {
	flag.Parse()

	if *hostid == "" {
		*hostid, _ = os.Hostname()
	}

	if *kubeconfig {
		config := ugatesvc.NewConf()
		_ = auth.NewAuth(config, *hostid, "m.webinf.info")
		gen, _ := config.Get("kube.json")
		fmt.Println(string(gen))
		return
	}


	cfgDir := "./"
	config := ugatesvc.NewConf(cfgDir, "./var/lib/dmesh/")

	authz := auth.NewAuth(config, *hostid, "m.webinf.info")
	ug := ugatesvc.New(config, authz, nil)

	hc = &http.Client{
		Transport: ug,
	}

	if *jwt != "" {
		decode(*jwt, *aud)
		return
	}

	if *aud != "" {
		fmt.Println(authz.VAPIDToken(*aud))
		return
	}

	if *netcat != "" {
		Netcat(ug, *netcat, *addr)
	}

	if *watch {
		watchNodes(ug)
		return
	}

	if *sub != "" {
		// Subscribe will add the given host to the list of nodes allowed to
		// send messages to this node, and generate a subscription similar with
		// browsers.
		// Value is the public key of the sender.
		subsc := subscribe(ug, *sub)
		fmt.Println(subsc)
		return
	}

	sendMessage(ug, *to, authz, *verbose, *data)
}

func Netcat(ug *ugatesvc.UGate, s string, via string) {
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

func subscribe(ug *ugatesvc.UGate, s string) string {
	// TODO - create local permission/host, generate one for remote
	// This doesn't require a running server, creates a file.
	return ""
}


// Send a message.
func sendMessage(ug *ugatesvc.UGate, toS string, vapid *auth.Auth, show bool,
	msg string) {

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

	if *pushService != "" {
		destURL = *pushService + "/push/"
		//hc = h2.InsecureHttp()
	} else {
		//hc = h2.NewSocksHttpClient("127.0.0.1:5224")
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

	} else if *verbose {
		dmpReq, _ := httputil.DumpRequest(req, true)
		fmt.Printf(string(dmpReq))
		dmp, _ := httputil.DumpResponse(res, true)
		fmt.Printf(string(dmp))
	}
}

// Known nodes and watchers
var watchers = map[uint64]*watcher{}

// Track a single node
type watcher struct {
	Node *ugate.DMNode

	Path []string

	// For crawling
	dirty bool

	base string
	ip6  *net.IPAddr

	idhex    string
	New      bool
	watching bool
}


// Connect to all known nodes and watch them.
// Uses the internal debug endpoint for getting connected nodes,
// and crawls the mesh.
func watchNodes(ug *ugatesvc.UGate) {
	ctx := context.Background()

	if *recurse {
		// Recursive list of all devices in the mesh.
		for {
			crawl() // Get nodes connected to our parent ( all directions )

			if !*watch {
				return // Just crawl.
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

							ug.DialMUX(ctx, "quic", w.Node, nil)
							w.watching = false
							log.Println("Watch close: ", w.ip6)
						}()
					}
				}
			}
			time.Sleep(10 * time.Second)
			//select {}
		}
		return
	}

	n0 := &ugate.DMNode{
		Addr: *addr,
	}

	ug.DialMUX(ctx, "quic", n0, nil)
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
	connectedNodes := map[uint64]*ugate.DMNode{}
	err = json.Unmarshal(cnodes, &connectedNodes)
	if err != nil {
		log.Println("HTTP_ERROR_RES", url, err, string(cnodes))
		return nil
	}

	if watchers != nil {
		peers := []string{}
		// Any node that is new, not yet found in the current watched list.
		newConnectedNodes := map[uint64]*watcher{}
		// Nodes already connected
		oldmap := map[uint64]*watcher{}
		for k, v := range connectedNodes {
			w, f := watchers[k]
			if !f {
				ip6 := toIP6(k)
				w = &watcher{Node: v, dirty: true, ip6: ip6,
					idhex: fmt.Sprintf("%x", k),
					Path:  path}
				w.New = true
				watchers[k] = w
				newConnectedNodes[k] = w
			} else {
				oldmap[k] = w
			}
			peers = append(peers, w.idhex)
		}
		if *verbose {
			log.Println("GET", url, len(connectedNodes), peers)
		}
	}

	return connectedNodes
}

// Scan all nodes for neighbors. Watch them if not watched already.
//
// - Will first mark all currently known nodes as 'dirty' and 'old'.
// - For each known node, get a list of connected nodes.
// - For each connected node, if still connected update and set dirty=false
// - If new nodes is found, add it as 'New' and 'dirty'.
//
// - repeat until all nodes are !dirty
// - at the end, the set of nodes that are new will be marked as New.
// -
//
func crawl() {
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

func (w *watcher) monitor(ug *ugatesvc.UGate, addr *net.IPAddr) {

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

// Decode a JWT.
// If crt is specified - verify it using that cert
func decode(jwt, aud string) {
	// TODO: verify if it's a VAPID
	parts := strings.Split(jwt, ".")
	p1b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println(string(p1b))

	scrt, _ := ioutil.ReadFile("server.crt")
	block, _ := pem.Decode(scrt)
	xc, _ := x509.ParseCertificate(block.Bytes)
	log.Printf("Cert subject: %#v\n", xc.Subject)
	pubk1 := xc.PublicKey


	h, t, txt, sig, _ := auth.JwtRawParse(jwt)
	log.Printf("%#v %#v\n", h, t)

	if h.Alg == "RS256" {
		rsak := pubk1.(*rsa.PublicKey)
		hasher := crypto.SHA256.New()
		hasher.Write(txt)
		hashed := hasher.Sum(nil)
		err = rsa.VerifyPKCS1v15(rsak,crypto.SHA256, hashed, sig)
		if err != nil {
			log.Println("Root Certificate not a signer")
		}
	}

}
