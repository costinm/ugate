// +build gvisor

package main

import (
	"io"
	"log"
	"os"

	"github.com/costinm/ugate/ext/gvisor"
	"github.com/costinm/ugate/pkg/udp"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/songgao/water"
)

func openTun(ifn string) (io.ReadWriteCloser, error) {
	config := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Persist: true,
		},
	}
	config.Name = ifn
	ifce, err := water.New(config)

	if err != nil {
		return nil, err
	}
	return ifce.ReadWriteCloser, nil
}

var tun io.ReadWriteCloser

func init() {
	initHooks = append(initHooks, func(ug *ugatesvc.UGate) startFunc {
		dev := os.Getenv("GVISOR")
		if dev == "" {
			return nil
		}
		fd, err := openTun(dev)
		if err != nil {
			return nil
		}
		tun = fd

		log.Println("Using gVisor tun", dev)

		return func(ug *ugatesvc.UGate) {
			tun := gvisor.NewTUNFD(fd,ug, ug.UDPHandler)
			udp.TransparentUDPWriter = tun
		}
	})
}
