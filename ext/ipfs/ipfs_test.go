package ipfs

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"testing"

	//ws "github.com/costinm/go-ws-transport"
	ws "github.com/libp2p/go-ws-transport"
	blankhost "github.com/libp2p/go-libp2p-blankhost"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/transport"
	tpt "github.com/libp2p/go-libp2p-core/transport"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	swarm "github.com/libp2p/go-libp2p-swarm"
	ma "github.com/multiformats/go-multiaddr"
)

const key = "CAESQDXW7-QhEhXWdgDUg7AvhlJU2eN-2IzMoDOWl_P271npGnwf4KUMcqufSakCfFi373F8C2HqINHxWalQwk3pVrc="

func TestBlankHost(t *testing.T) {
	kb, _ := base64.URLEncoding.DecodeString(key)

	priv, _ := ic.UnmarshalPrivateKey(kb)

	peerID, err := peer.IDFromPrivateKey(priv)

	tr, err := initTransport("5555", priv)
	if err != nil {
		t.Fatal(err)
	}
	ps := pstoremem.NewPeerstore()
	// BlankHost is used in testing - usually with a swarm for transport
	// Options can provide a non-null ConnMgr
	// In addition it has a 'mux', bus, network - currently Swarm is the only non-mock impl

	// The 'swarm' is responsible for handshake in user-space using the protocol ID.
	// No metadata.
	n := swarm.NewSwarm(context.Background(), peerID, ps, nil)
	n.AddTransport(tr)
	h := blankhost.NewBlankHost(n)

	log.Println("XXX", h)
}

func initTransport(port string, priv ic.PrivKey) (transport.Transport, error) {
	addr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s/ws", port))
if err != nil {
		return nil, err
	}

	peerID, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return nil, err
	}

	t := ws.New(nil) //NewSPDY(priv, nil, nil)

	ln, err := t.Listen(addr)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Listening. Now run: go run cmd/client/main.go %s %s\n", ln.Multiaddr(), peerID)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return nil, err
		}
		log.Printf("Accepted new connection from %s (%s)\n", conn.RemotePeer(), conn.RemoteMultiaddr())
		log.Println(conn.RemotePeer().String(), conn.RemoteMultiaddr().String(),
			conn.RemotePublicKey())

		go func() {
			if err := handleConn(conn); err != nil {
				log.Printf("handling conn failed: %s", err.Error())
			}
		}()
	}

	return t, nil

}


func handleConn(conn tpt.CapableConn) error {
	str, err := conn.AcceptStream()
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(str)
	if err != nil {
		return err
	}
	log.Printf("Received: %s\n", data)
	if _, err := str.Write([]byte(data)); err != nil {
		return err
	}
	return str.Close()
}
