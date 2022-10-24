package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/costinm/hbone"
	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/cfgfs"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/costinm/ugate/test"
)

func TestFull(t *testing.T) {
	// Fixed key, config from filesystem. Base is 14000
	alice, err := ugatesvc.Run(cfgfs.NewConf("testdata/alice/"), nil)
	if err != nil {
		t.Fatal(err)
	}
	alice.StartListener(&hbone.Listener{
		Address: fmt.Sprintf("0.0.0.0:%d", 14011),
		//Protocol:  "tls",
		Handler: &ugatesvc.EchoHandler{},
	})

	// In memory config store. All options
	config := cfgfs.NewConf()
	bob, err := ugatesvc.Run(config, &ugate.MeshSettings{
		BasePort: 14100,
	})
	bob.StartListener(&hbone.Listener{
		Address: fmt.Sprintf("0.0.0.0:%d", 14111),
		//Protocol:  "tls",
		Handler: &ugatesvc.EchoHandler{},
	})

	// Client gateways - don't listen.
	cl1 := ugatesvc.New(nil, nil, nil)
	cl2 := ugatesvc.New(nil, nil, nil)

	con, err := cl1.DialContext(context.Background(), "tcp", "127.0.0.1:14011")
	if err != nil {
		t.Fatal(err)
	}
	_, err = test.CheckEcho(con, con)

	con, err = cl2.DialContext(context.Background(), "tcp", "127.0.0.1:14111")
	if err != nil {
		t.Fatal(err)
	}
	_, err = test.CheckEcho(con, con)

}
