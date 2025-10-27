package cmd

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
	"strings"
	"time"

	"github.com/costinm/meshauth"
)

// TODO: rewrite using K8s API style

var (
	addr = flag.String("addr", "127.0.0.1:15007", "address:port for the node")

	watch = flag.Bool("watch", false, "Watch")

	recurse = flag.Bool("r", false, "Crawl and watch all possible nodes")
)

var hc *http.Client

type Watcher struct {
	Debug bool
	ug    *meshauth.Mesh
	nodes map[uint64]*NodeWatcher
}

// Create a HBONE tunnel to a given URL.
//
// Current client is authenticated using local credentials, or a kube.json file. If no kube.json is found, one
// will be generated.
//
// Example:
// ssh -v -o ProxyCommand='wp -nc https://c1.webinf.info:443/dm/PZ5LWHIYFLSUZB7VHNAMGJICH7YVRU2CNFRT4TXFFQSXEITCJUCQ:22'  root@PZ5LWHIYFLSUZB7VHNAMGJICH7YVRU2CNFRT4TXFFQSXEITCJUCQ
func NewWatcher(ug *meshauth.Mesh) {

	hc = http.DefaultClient
	w := Watcher{ug: ug,
		nodes: map[uint64]*NodeWatcher{}}
	w.watchNodes()
}

// Track a single node
type NodeWatcher struct {
	Node *meshauth.Dest

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
func (wa *Watcher) watchNodes() {
	//ctx := context.Background()

	//if *recurse {
	//	// Recursive list of all devices in the mesh.
	//	for {
	//		wa.crawl() // Get nodes connected to our parent ( all directions )
	//
	//		if !*watch {
	//			return // Just crawl.
	//		} else {
	//			//if *dev == "*" || *dev == "" {
	//			//	go msgs.DefaultMux.MonitorNode(hc, nil, nil)
	//			//}
	//			for _, w := range wa.nodes {
	//				//if w.idhex == *dev || *dev == "*" {
	//				if !w.watching {
	//					w.watching = true
	//
	//					go func() {
	//						log.Println("Watch: ", w.ip6)
	//
	//						wa.ug.DialMUX(ctx, "quic", w.Node, nil)
	//						w.watching = false
	//						log.Println("Watch close: ", w.ip6)
	//					}()
	//				}
	//			}
	//		}
	//		time.Sleep(10 * time.Second)
	//		//select {}
	//	}
	//	return
	//}
	//
	//n0 := & meshauth.Dest{
	//		Addr: *addr,
	//}
	//
	//wa.ug.DialMUX(ctx, "quic", n0, nil)
}

// Get neighbors from a node. Side effect: updates the watchers table.
func (wa *Watcher) neighbors(url string, path []string) map[uint64]*meshauth.Dest {
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
	connectedNodes := map[uint64]*meshauth.Dest{}
	err = json.Unmarshal(cnodes, &connectedNodes)
	if err != nil {
		log.Println("HTTP_ERROR_RES", url, err, string(cnodes))
		return nil
	}

	if wa.nodes != nil {
		peers := []string{}
		// Any node that is new, not yet found in the current watched list.
		newConnectedNodes := map[uint64]*NodeWatcher{}
		// Nodes already connected
		oldmap := map[uint64]*NodeWatcher{}
		for k, v := range connectedNodes {
			w, f := wa.nodes[k]
			if !f {
				ip6 := toIP6(k)
				w = &NodeWatcher{Node: v, dirty: true, ip6: ip6,
					idhex: fmt.Sprintf("%x", k),
					Path:  path}
				w.New = true
				wa.nodes[k] = w
				newConnectedNodes[k] = w
			} else {
				oldmap[k] = w
			}
			peers = append(peers, w.idhex)
		}
		log.Println("GET", url, len(connectedNodes), peers)

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
func (wa *Watcher) crawl() {
	for _, v := range wa.nodes {
		v.dirty = true
		v.New = false
	}

	// Will update watchers
	wa.neighbors("http://127.0.0.1:5227/dmesh/ip6", nil)

	more := true

	for more {
		more = false
		for _, v := range wa.nodes {
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
			wa.neighbors("http://"+net.JoinHostPort(v.ip6.String(), "5227")+"/dmesh/ip6", append(v.Path, v.idhex))
			v.dirty = false
		}
	}

	for _, v := range wa.nodes {
		if v.New {
			log.Println("Node", v.idhex, v.Path, v.Node)
		}
	}
}

func toIP6(k uint64) *net.IPAddr {
	ip6, _ := net.ResolveIPAddr("ip6", "fd00::")
	binary.BigEndian.PutUint64(ip6.IP[8:], k)
	return ip6
}

func (w *NodeWatcher) monitor(ug *meshauth.Mesh, addr *net.IPAddr) {

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
