//go:build !nolwip
// +build !nolwip

package ugated

import (
	"io"
	"log"
	"os"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/ext/lwip"
	"github.com/costinm/ugate/pkg/udp"
	"github.com/songgao/water"
)

func openTunLWIP(ifn string) (io.ReadWriteCloser, error) {
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

// init() will load LWIP if dmesh0 is present
//
//	sudo   ip tuntap add dev dmesh0 mode tun user build
func init() {
	ugate.Modules["lwip"] = func(ug *ugate.UGate) {
		dev := os.Getenv("LWIP")
		if dev == "" {
			dev = "dmesh0"
			//return nil
		}
		fd, err := openTunLWIP(dev)
		if err != nil {
			return
		}

		log.Println("Using LWIP tun", dev)

		t := lwip.NewTUNFD(fd, ug, ug)
		// Use the TUN for transparent UDP write ?
		udp.TransparentUDPWriter = t
	}
}
