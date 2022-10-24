///// +build gvisor

package ugated

import (
	"io"
	"log"
	"os"

	"github.com/costinm/ugate/pkg/udp"
	"github.com/costinm/ugate/pkg/ugatesvc"
	gv "github.com/costinm/ugate/ugated/pkg/gvisor"
	"github.com/songgao/water"
	//"golang.zx2c4.com/wireguard/tun/netstack"
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

var tunIO io.ReadWriteCloser

func init() {
	ugatesvc.InitHooks = append(ugatesvc.InitHooks, func(ug *ugatesvc.UGate) ugatesvc.StartFunc {
		dev := os.Getenv("GVISOR")
		if dev == "" {
			return nil
		}
		fd, err := openTun(dev)
		if err != nil {
			return nil
		}
		tunIO = fd

		log.Println("Using gVisor tun", dev)

		//if true {
		//	return func(ug *ugatesvc.UGate) {
		//		tunx, tnet, err := netstack.CreateNetTUN(
		//			[]netip.Addr{netip.MustParseAddr("192.168.4.29")},
		//			[]netip.Addr{netip.MustParseAddr("8.8.8.8")},
		//			1420)
		//
		//		if err != nil {
		//			log.Panic(err)
		//		}
		//}
		//
		//}
		return func(ug *ugatesvc.UGate) {
			tun := gv.NewTUNFD(fd, ug, ug.UDPHandler)
			udp.TransparentUDPWriter = tun
		}
	})
}
