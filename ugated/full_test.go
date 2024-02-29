package ugated

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/costinm/ssh-mesh/nio"
	"github.com/costinm/ugate"
	"github.com/costinm/ugate/pkg/echo"
	"github.com/costinm/ugate/pkg/test"
	mqtt "github.com/mochi-co/mqtt/server"

	"github.com/costinm/meshauth"
	"github.com/golang/protobuf/jsonpb"
)

// This directory has its own go.mod with all the dependencies, so it can run more complete tests.
//

func TestFull(t *testing.T) {
	alice, err := test.NewClientNode(&meshauth.MeshCfg{CertDir: "../testdata/alice"},
		&ugate.MeshSettings{BasePort: 12000})
	if err != nil {
		t.Fatal(err)
	}

	alice.StartListener(&meshauth.PortListener{
		Address:  fmt.Sprintf("0.0.0.0:%d", 14011),
		Protocol: "echo",

		//Handler: &ugatesvc.EchoHandler{},
	})

	mb := meshauth.NewMeshAuth(&meshauth.MeshCfg{
		CertDir: "testdata/bob",
		Listeners: map[string]*meshauth.PortListener{
			"echo": &meshauth.PortListener{
				Address:  fmt.Sprintf("0.0.0.0:%d", 14111),
				Protocol: "echo",
			},
		},
	})
	bob := ugate.New(mb, &ugate.MeshSettings{
		BasePort: 14100,
	})
	bob.ListenerProto["echo"] = echo.EchoPortHandler

	bob.Start()

	// Client gateways - don't listen.
	cl1 := ugate.New(nil, nil)
	cl2 := ugate.New(nil, nil)

	t.Run("Direct", func(t *testing.T) {
		// Direct connection, no crypto - verify the echo handlers on bob and alice
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
	})

	t.Run("mqtt", func(t *testing.T) {
		mqtt.New()
	})

}

// Requires a GSA (either via GOOGLE_APPLICATION_CREDENTIALS, gcloud config, metadata) with hub and
// container access.
// Requires a kube config - the default cluster should be in same project.
//
// Will verify kube config loading and queries to hub and gke.
func TestURest(t *testing.T) {
	ctx, cf := context.WithCancel(context.Background())
	defer cf()

	// No certificate by default, but the test may be run with different params
	aliceCfg := &ugate.MeshSettings{}

	aliceID := meshauth.NewMeshAuth(nil)

	// TODO: test FromEnv with different options

	alice := ugate.New(aliceID, aliceCfg)

	//otel.InitProm(hb)
	//otel.OTelEnable(hb)
	//otelC := setup.FileExporter(ctx, os.Stderr)
	//defer otelC()

	//gcp.InitDefaultTokenSource(ctx, alice)

	// InitOptionalModules credentials and discovery server.
	//k8sService, err := k8s.InitK8S(ctx, alice)
	//if err != nil {
	//	t.Fatal(err)
	//}

	//t.Run("get-cert", func(t *testing.T) {
	//	err = urpc.GetCertificate(ctx, aliceID, istiodC)
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//})

	//t.Run("xdsc-clusters", func(t *testing.T) {
	//	err = xdsTest(t, ctx, istiodC, alice)
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//})

	// Test for 'system' streams
	//t.Run("xdsc-system", func(t *testing.T) {
	//	system := ugate.New(nil, nil)
	//	//gcp.InitDefaultTokenSource(ctx, system)
	//
	//	catokenSystem := k8sService.NewK8STokenSource("istio-ca")
	//	catokenSystem.Namespace = "istio-system"
	//	catokenSystem.KSA = "default"
	//
	//	istiodSystem := system.AddCluster(&ugate.MeshCluster{
	//		Dest: meshauth.Dest{Addr: istiodC.Addr, SNI: "istiod.istio-system.svc",
	//			CACertPEM:     istiodC.CACertPEM,
	//			TokenProvider: catokenSystem,
	//		},
	//		ID: "istiod",
	//	})
	//
	//	turl := "istio.io/debug"
	//	systemXDS, err := urpc.DialContext(ctx, "", &urpc.Config{
	//		Cluster: istiodSystem,
	//		HBone:   system,
	//		Meta: map[string]interface{}{
	//			"SERVICE_ACCOUNT": "default",
	//			"NAMESPACE":       "istio-system",
	//			"GENERATOR":       "event",
	//			// istio.io/debug
	//		},
	//		Namespace: "istio-system",
	//		InitialDiscoveryRequests: []*xds.DiscoveryRequest{
	//			{TypeUrl: turl, ResourceNames: []string{"syncz"}},
	//		},
	//	})
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//	err = systemXDS.Send(&xds.DiscoveryRequest{
	//		TypeUrl:       turl,
	//		ResourceNames: []string{"syncz"},
	//	})
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//
	//	_, err = systemXDS.Wait(turl, 3*time.Second)
	//
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//	dr := systemXDS.Received[turl]
	//	log.Println(dr.Resources)
	//})

	// Used for GCP tokens and calls.
	//projectNumber := hb.GetEnv("PROJECT_NUMBER", "")
	//projectId := alice.GetEnv("PROJECT_ID", "")

	//hb.ProjectId = projectId

	//t.Run("meshca-cert", func(t *testing.T) {
	//	meshCAID := meshauth.NewMeshAuth(nil)
	//
	//	meshca := alice.AddCluster(&ugate.MeshCluster{
	//		Dest: meshauth.Dest{Addr: "meshca.googleapis.com:443",
	//			TokenSource: "sts"},
	//	})
	//	err := urpc.GetCertificate(ctx, meshCAID, meshca)
	//
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//	log.Println("Cert: ", meshCAID.String())
	//})

	tok1, err := alice.AuthProviders["gsa"](ctx, "https://example.com")
	if err != nil {
		t.Fatal("Failed to load k8s", err)
	}
	tok1J := meshauth.DecodeJWT(tok1)
	log.Println("Tok:", tok1J)

	//t.Run("watch", func(t *testing.T) {
	//	req := k8sService.RequestAll(ctx, "istio-system", "configmap", "", nil,
	//		"1")
	//	tr, err := k8sService.HttpClient.Do(req)
	//
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//	if tr.Status != "200" {
	//		t.Fatal("Response statuds ", tr.Status, tr.Header)
	//	}
	//	fr := urpc.NewFromStream(nil)
	//	for {
	//		bb, err := fr.Recv4(tr.Body)
	//		if err != nil {
	//			t.Fatal(err)
	//		}
	//		log.Println(string(bb.Bytes()))
	//
	//	}
	//})

}

var marshal = &jsonpb.Marshaler{OrigName: true, Indent: "  "}

//func xdsTest(t *testing.T, ctx context.Context, istiodC *ugate.MeshCluster, ns *ugate.UGateHandlers) error {
//	xdsc, err := urpc.DialContext(ctx, "", &urpc.Config{
//		Cluster: istiodC,
//		HBone:   ns,
//		ResponseHandler: func(con *urpc.ADSC, r *xds.DiscoveryResponse) {
//			log.Println("DR:", r.TypeUrl, r.VersionInfo, r.Nonce, len(r.Resources))
//			for _, l := range r.Resources {
//				b, err := marshal.MarshalToString(l)
//				if err != nil {
//					log.Printf("Error in LDS: %v", err)
//				}
//
//				if false {
//					log.Println(b)
//				}
//			}
//		},
//	})
//
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	for {
//		res := <-xdsc.Updates
//		if res == "eds" {
//			break
//		}
//	}
//
//	for _, c := range xdsc.Endpoints {
//
//		if len(c.Endpoints) > 0 && len(c.Endpoints[0].LbEndpoints) > 0 {
//			log.Println(c.ClusterName, c.Endpoints[0].LbEndpoints[0].Host.Address)
//		}
//		//log.Println(prototext.MarshalOptions{Multiline: true}.Format(&c))
//	}
//
//	return err
//}


// UGate tests without a CA ( peer to peer )
func TestNoCA(t *testing.T) {
	// xx07 -> BTS port
	// xx12 -> plain text echo
	// "/dm/" -> POST-based tunnel ( more portable than CONNECT )

	// Bob connected to Carol
	bob := test.NewTestNode(test.BobMeshAuthCfg, &ugate.MeshSettings{
		BasePort: 6100})

	// Alice connected to Bob
	alice := test.NewTestNode(test.AliceMeshAuthCfg, &ugate.MeshSettings{
		BasePort: 600,
		Clusters: map[string]*ugate.MeshCluster{
			"bob": {
				Dest: meshauth.Dest{Addr: "127.0.0.1:6107"},
			},
		},
	})

	//http2.DebugGoroutines = true

	// No other config - should start only the Hbone server
	bob.Start()

	t.Run("Echo-tcp", func(t *testing.T) {
		ab, err := alice.DialContext(context.Background(), "tcp",
			fmt.Sprintf("127.0.0.1:%d", 6112))
		if err != nil {
			t.Fatal(err)
		}
		res, err := test.CheckEcho(ab, ab)
		if err != nil {
			t.Fatal(err)
		}
		log.Println("Result ", res)
	})

	// TLS Echo server, on 6111
	t.Run("Echo-tls", func(t *testing.T) {
		ab, err := alice.DialContext(context.Background(), "tls",
			fmt.Sprintf("127.0.0.1:%d", 6111))
		if err != nil {
			t.Fatal(err)
		}
		// TODO: verify the identity, cert, etc
		res, err := test.CheckEcho(ab, ab)
		if err != nil {
			t.Fatal(err)
		}
		mc := ab.(nio.Stream)
		log.Println("Result ", res, mc)
	})

	// This is a H2 (BTS) request that is forwarded to a TCP stream handler.
	// Alice -BTS-> Bob -TCP-> Carol
	//t.Run("H2-egress", func(t *testing.T) {
	//	i, o := io.Pipe()
	//	r, _ := http.NewRequest("POST", "https://127.0.0.1:6107/dm/"+"127.0.0.1:6112", i)
	//	res, err := alice.RoundTrip(r)
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//
	//	res1, err := CheckEcho(res.Body, o)
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//
	//	log.Println(res1, alice, bob)
	//})

}

