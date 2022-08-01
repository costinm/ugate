package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/auth"
	"github.com/costinm/ugate/pkg/ugatesvc"
	msgs "github.com/costinm/ugate/webpush"
)

var (
	addr = flag.String("addr", "127.0.0.1:15007", "address:port for the node")

	watch = flag.Bool("watch", false, "Watch")

	recurse = flag.Bool("r", false, "Crawl and watch all possible nodes")
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
func main() {
	flag.Parse()

	config := ugatesvc.NewConf("./", "./var/lib/dmesh/")
	authz := auth.NewAuth(config, "", "")

	ug := ugatesvc.New(config, authz, nil)

	hc = &http.Client{
		Transport: ug,
	}

	watchNodes(ug)
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
