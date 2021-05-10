// +build lwip

package bootstrapx

import (
	"io"
	"log"
	"os"

	"github.com/costinm/ugate/ext/lwip"
	"github.com/costinm/ugate/pkg/udp"
	"github.com/costinm/ugate/pkg/ugatesvc"
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

func init() {
	initHooks = append(initHooks, func(ug *ugatesvc.UGate) startFunc {
		dev := os.Getenv("LWIP")
		if dev == "" {
			return nil
		}
		fd, err := openTunLWIP(dev)
		if err != nil {
			return nil
		}

		log.Println("Using LWIP tun", dev)

		return func(ug *ugatesvc.UGate) {
			tun := lwip.NewTUNFD(fd,ug, ug.UDPHandler)
			udp.TransparentUDPWriter = tun
		}
		return func(ug *ugatesvc.UGate) {
		}
	})
}
