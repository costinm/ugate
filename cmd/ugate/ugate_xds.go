// +build !MIN

package main

import (
	"net/http"

	"github.com/costinm/ugate/ext/xds"
	"github.com/costinm/ugate/pkg/ugatesvc"
	"github.com/costinm/ugate/webpush"
	"google.golang.org/grpc"
)

// XDS and gRPC dependencies. Enabled for interop with Istio/XDS.
func init() {
	initHooks = append(initHooks, func(ug *ugatesvc.UGate) startFunc {
		gs := xds.NewXDS(webpush.DefaultMux)
		grpcS := grpc.NewServer()
		ug.Mux.HandleFunc("/envoy.service.discovery.v3.AggregatedDiscoveryService/StreamAggregatedResources", func(writer http.ResponseWriter, request *http.Request) {
			grpcS.ServeHTTP(writer, request)
		})
		xds.RegisterAggregatedDiscoveryServiceServer(grpcS, gs)

		// TODO: register for config change, connect to upstream
		return nil
	})
}
