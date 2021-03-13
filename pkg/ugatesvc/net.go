package ugatesvc

import (
	"time"

	"github.com/costinm/ugate"
)

// Keep track of network info - known nodes, active connections, routing.




func NewDMNode() *ugate.DMNode {
	now := time.Now()
	return &ugate.DMNode{
		Labels:       map[string]string{},
		FirstSeen:    now,
		LastSeen:     now,
		NodeAnnounce: &ugate.NodeAnnounce{},
	}
}

