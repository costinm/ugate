package ugatesvc

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/costinm/ugate"
)

// Reverse tunnel: create a persistent connection to a gateway, and
// accept connections over that connection.
//
// The gateway registers the current endpoint with it's own IP:port
// (for example WorkloadEntry or Endpoint or in-memory ), and forwards accepted requests over the
// established connection.

// UpdateReverseAccept updates the upstream accept connections, based on config.
// Should be called when the config changes
func (t *H2Transport) UpdateReverseAccept() {
	ev := make(chan string)
	for addr, key := range t.ug.Config.H2R {
		// addr is a hostname
		dm := t.ug.GetOrAddNode(addr)
		if dm.Addr == "" {
			if key == "" {
				dm.Addr = net.JoinHostPort(addr, "443")
			} else {
				dm.Addr = net.JoinHostPort(addr, "15007")
			}
		}

		go t.maintainPinnedConnection(dm, ev)
	}
	<-ev
	log.Println("maintainPinned connected for ", t.ug.Auth.VIP6)

}

// Reverse Accept dials a connection to addr, and registers a H2 SERVER
// conn on it. The other end will register a H2 client, and create streams.
// The client cert will be used to associate incoming streams, based on config or direct mapping.
// TODO: break it in 2 for tests to know when accept is in effect.
func (t *H2Transport) maintainPinnedConnection(dm *ugate.Cluster, ev chan string) {
	// maintain while the host is in the 'pinned' list
	if _, f := t.ug.Config.H2R[dm.ID]; !f {
		return
	}

	//ctx := context.Background()
	if dm.Backoff == 0 {
		dm.Backoff = 1000 * time.Millisecond
	}

	ctx := context.TODO()
	//ctx, ctxCancel := context.WithTimeout(ctx, 5*time.Second)
	//defer ctxCancel()

	protos := []string{"quic", "h2r"}
	var err error
	var muxer ugate.Muxer
	for _, k := range protos {
		muxer, err = t.ug.DialMUX(ctx, k, dm, nil)
		if err == nil {
			break
		}
	}
	if err == nil {
		log.Println("UP: ", dm.Addr, muxer)
		// wait for mux to be closed
		dm.Backoff = 1000 * time.Millisecond
		return
	}

	log.Println("UP: err", dm.Addr, err, dm.Backoff)
	// Failed to connect
	if dm.Backoff < 15*time.Minute {
		dm.Backoff = 2 * dm.Backoff
	}

	time.AfterFunc(dm.Backoff, func() {
		t.maintainPinnedConnection(dm, ev)
	})

	// p := str.TLS.NegotiatedProtocol
	//if p == "h2r" {
	//	// Old code used the 'raw' TLS connection to create a  server connection
	//	t.h2Server.ServeConn(
	//		str,
	//		&http2.ServeConnOpts{
	//			Handler: t, // Also plain text, needs to be upgraded
	//			Context: str.Context(),
	//
	//			//Context: // can be used to cancel, pass meta.
	//			// h2 adds http.LocalAddrContextKey(NetAddr), ServerContextKey (*Server)
	//		})
	//}
}
