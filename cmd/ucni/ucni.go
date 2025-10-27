package ucni

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/040"
	"github.com/containernetworking/cni/pkg/version"
)

/*
  Minimal delegating CNI. This is a binary leaving on the host, under
	/opt/cni/bin/ucni

  K8S kubelet will call it with ADD, DEL and CHECK commands when a container is
  created. Docker and other container runtimes will also call it.

  Instead of doing the network setup - it will just pass all info to a container
  or service.

  - TODO: Will attempt to connect to a file (/var/run/cni/ucni.sock ) or port.



*/

type Config struct {
	types.NetConf
	ContIPv4 net.IPNet `json:"-"`
	ContIPv6 net.IPNet `json:"-"`
}

func main() {

	skel.PluginMain(
		func(add *skel.CmdArgs) error {
			return handle("ADD", add)
		},
		func(add *skel.CmdArgs) error {
			log.Print("CHECK:", add)
			return nil
		},
		func(add *skel.CmdArgs) error {
			log.Print("DEL:", add)
			return nil
		},
		version.PluginSupports("v1"),
		fmt.Sprintf("CNI %s plugin %s", "uNCI", "v1"),
	)

	os.Exit(0)
}

func handle(cmd string, params *skel.CmdArgs) error {
	conf := &Config{}
	err := json.Unmarshal(params.StdinData, conf)
	if err != nil {
		return err
	}
	var result *current.Result
	if conf.RawPrevResult != nil {
		var err error
		if err = version.ParsePrevResult(&conf.NetConf); err != nil {
			return fmt.Errorf("could not parse prevResult: %v", err)
		}

		result, err = current.NewResultFromResult(conf.PrevResult)
		if err != nil {
			return fmt.Errorf("could not convert result to current version: %v", err)
		}
		for _, ip := range result.IPs {
			if ip.Version == "6" && conf.ContIPv6.IP != nil {
				continue
			} else if ip.Version == "4" && conf.ContIPv4.IP != nil {
				continue
			}

			// Skip known non-sandbox interfaces
			if ip.Interface != nil {
				intIdx := *ip.Interface
				if intIdx >= 0 &&
					intIdx < len(result.Interfaces) &&
					(result.Interfaces[intIdx].Name != params.IfName ||
						result.Interfaces[intIdx].Sandbox == "") {
					continue

				}
			}

			switch ip.Version {
			case "6":
				conf.ContIPv6 = ip.Address
			case "4":
				conf.ContIPv4 = ip.Address
			}
		}
	}

	data := string(params.StdinData)
	log.Print(cmd, params.IfName, params.Netns, params.ContainerID, params.Args, params.Path, data, result)
	return nil
}
