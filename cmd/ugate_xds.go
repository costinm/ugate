//go:build !nogrpc

package cmd

import (
	"expvar"

	"github.com/costinm/grpc-mesh/pkg/echo"
	"github.com/costinm/grpc-mesh/pkg/istioca"
	"github.com/costinm/grpc-mesh/pkg/otel"
	"github.com/costinm/grpc-mesh/pkg/xds"
	"github.com/costinm/meshauth/pkg/certs"
	"github.com/costinm/ugate/appinit"
)

func init() {
	appinit.RegisterT("istioca", &istioca.IstioCA{})
	appinit.RegisterT("gRPCEcho", &echo.Echo{})
	appinit.RegisterN("xds", xds.NewXDSServer)

	// OTel endpoints (TODO: move to separate file)
	//mux.Handle(traceconnect.NewTraceServiceHandler(&otel.OTelSvc{}))
	//mux.Handle(logsconnect.NewLogsServiceHandler(&otel.OTelSvcLogs{}))
	appinit.RegisterT("otel", &otel.OTelSvc{})
	appinit.RegisterT("otelLogs", &otel.OTelSvcLogs{})
	//mux.Handle(v2connect.NewLoadReportingServiceHandler(xds.NewLRS()))
	appinit.RegisterN("lrs", xds.NewLRS)

	expvar.Publish("modules.xds.client", expvar.Func(func() any {
		// Namespace:  *ns,
		// Workload:   *pod,
		// XDSHeaders: nil,
		// IP:         ip,
		// XDS:        *IstiodAddr,
		// Context:    context.Background(),
		return &xds.XDSConfig{}
	}))

	expvar.Publish("modules.istio.ca", expvar.Func(func() any {
		return certs.NewCerts()
	}))

	//go func() {
	//	err := x.RunDelta("ptr")
	//	if err != nil {
	//		log.Fatalln(err)
	//	}
	//}()

	//err := x.RunFull("cluster")
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//err := x.RunDelta("cluster")
	// err := x.RunDelta("ptr")
	// if err != nil {
	// 	log.Fatalln(err)
	// }

	// caDir := os.Getenv("CA_DIR")
	// if caDir == "" {
	// 	caDir = "."
	// }
	// // Operate in CA root mode
	// ca := certs.NewCerts()
	// ca.BaseDir = caDir
	// ca.Provision(context.Background())
	// if ca.LoadTime.IsZero() {
	// 	ca.Save(context.Background(), caDir)
	// }

}
