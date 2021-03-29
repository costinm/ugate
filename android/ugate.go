package android

import (
	"encoding/base64"
	"log"
	"os"
	"time"

	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/local"
	"github.com/costinm/ugate/pkg/msgs"
	"github.com/costinm/ugate/pkg/udp"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/costinm/ugate/pkg/auth"
)
/*
	gomobile bindings

- called 2x, once with lang=java and once with lang=go

- Signed integer and floating point types.

- String and boolean types.

- Byte slice types. Note that byte slices are passed by reference,
  and support mutation.

- Any function type all of whose parameters and results have
  supported types. Functions must return either no results,
  one result, or two results where the type of the second is
  the built-in 'error' type.

- Any interface type, all of whose exported methods have
  supported function types.

- Any struct type, all of whose exported methods have
  supported function types and all of whose exported fields
  have supported types.

 */


// Adapter from func to interface
type HandlerCallbackFunc func(cmdS string, data []byte)

type MessageHandler interface {
	Handle(topic string, data []byte)
}

var (
	ld *local.LLDiscovery
	gw *ugatesvc.UGate
	udpGate = udp.NewUDPGate(nil, nil)
)

// Called to inject a message into Go impl
func Send(cmdS string, data []byte) {
	switch cmdS {
	case "r":
		// refresh networks
		log.Println("UDS: refresh network (r)")
		go func() {
			time.Sleep(2 * time.Second)
			ld.RefreshNetworks()
		}()

		// TODO: P - properties, json
		// CON - STOP/START - set connected WIFI
		//
	}
}

// Android and device version of DMesh.
func InitDmesh(callbackFunc MessageHandler) {
	log.Print("Starting native process pwd=", os.Getenv("PWD"), os.Environ())

	// SYSTEMSERVERCLASSPATH=/system/framework/services.jar:/system/framework/ethernet-service.jar:/system/framework/wifi-service.jar:/system/framework/com.android.location.provider.jar
	// PATH=/sbin:/system/sbin:/system/bin:/system/xbin:/odm/bin:/vendor/bin:/vendor/xbin
	// STORAGE=/storage/emulated/0/Android/data/com.github.costinm.dmwifi/files
	// ANDROID_DATA=/data
	// ANDROID_SOCKET_zygote_secondary=12
	// ASEC_MOUNTPOINT=/mnt/asec
	// EXTERNAL_STORAGE=/sdcard
	// ANDROID_BOOTLOGO=1
	// ANDROID_ASSETS=/system/app
	// BASE=/data/user/0/com.github.costinm.dmwifi/files
	// ANDROID_STORAGE=/storage
	// ANDROID_ROOT=/system
	// DOWNLOAD_CACHE=/data/cache
	// BOOTCLASSPATH=/system/framework/core-oj.jar:/system/framework/core-libart.jar:/system/framework/conscrypt.jar:/system/framework/okhttp.jar:/system/framework/bouncycastle.jar:/system/framework/apache-xml.jar:/system/framework/ext.jar:/system/framework/framework.jar:/system/framework/telephony-common.jar:/system/framework/voip-common.jar:/system/framework/ims-common.jar:/system/framework/android.hidl.base-V1.0-java.jar:/system/framework/android.hidl.manager-V1.0-java.jar:/system/framework/framework-oahl-backward-compatibility.jar:/system/framework/android.test.base.jar:/system/framework/com.google.vr.platform.jar]

	cfgf := os.Getenv("BASE")
	if cfgf == "" {
		cfgf = os.Getenv("HOME")
		if cfgf == "" {
			cfgf = os.Getenv("TEMPDIR")
		}
		if cfgf == "" {
			cfgf = os.Getenv("TMP")
		}
		if cfgf == "" {
			cfgf = "/tmp"
		}
	}

	cfgf += "/"

	// File-based config
	config := ugatesvc.NewConf(cfgf)

	//meshH := "v.webinf.info:5222" // ugatesvc.Conf(config, "MESH", "v.webinf.info:5222")

	// Init or load certificates/keys
	authz := auth.NewAuth(config, os.Getenv("HOSTNAME"), "v.webinf.info")
	msgs.DefaultMux.Auth = authz

	gcfg := &ugate.GateCfg{
		BasePort: 15000,
	}

	GW := ugatesvc.NewGate(nil, authz, gcfg, nil)

	// HTTPGate - common structures
	//GW := mesh.New(authz, nil)

	// SSH transport + reverse streams.
	//sshg := sshgate.NewSSHGate(GW, authz)
	//GW.SSHGate = sshg
	//sshg.InitServer()
	//sshg.ListenSSH(":5222")
	//
	//// Connect to a mesh node
	//if meshH != "" {
	//	GW.Vpn = meshH
	//	go sshgate.MaintainVPNConnection(GW)
	//}

	// Local discovery interface - multicast, local network IPs
	ld := local.NewLocal(GW, authz)
	go ld.PeriodicThread()
	local.ListenUDP(ld)

	GW.Mux.HandleFunc("/dmesh/ll/if", ld.HttpGetLLIf)


	//h2s, err := h2.NewTransport(authz)
	//if err != nil {
	//	log.Fatal(err)
	//}

	// DNS capture, interpret the names, etc
	// Off until DNS moved to smaller package.
	//dnss, _ := dns.NewDmDns(5223)
	//GW.DNS = dnss
	//net.DefaultResolver.PreferGo = true
	//net.DefaultResolver.Dial = dns.DNSDialer(5223)

	//udpNat  := udp.NewUDPGate(GW)
	//udpNat.DNS = dnss

	//hgw := httpproxy.NewHTTPGate(GW, h2s)
	//hgw.HttpProxyCapture("localhost:5204")

	// Start a basic UI on the debug port
	//	u, _ := ui.NewUI(GW, h2s, hgw, ld)

	//udpNat.InitMux(h2s.LocalMux)

	//// Periodic registrations.
	//m.Registry.RefreshNetworksPeriodic()

	log.Printf("Loading with VIP6: %v ID64: %s\n",
		authz.VIP6,
		base64.RawURLEncoding.EncodeToString(authz.VIP6[8:]))

	//dnss.Serve()
	//err = http.ListenAndServe("localhost:5227", u)
	//if err != nil {
	//	log.Println(err)
	//}

	// TODO: mux adapter, so we can exchange messages.


}

