package local_discovery

import (
	"log"
	"testing"
	"time"
)

func TestDiscovery(t *testing.T) {
	d := &LLDiscovery{}
	err := d.Start()
	if err != nil {
		t.Fatal(err)
	}

	// Expect another node on the network - otherwise hard to discover.
	time.Sleep(1 * time.Second)

	log.Println(d.ActiveInterfaces)
	log.Println(d.Nodes)
}
