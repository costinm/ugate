module github.com/costinm/ugate/ext/xds

go 1.16

replace github.com/costinm/ugate => ../../

replace github.com/costinm/ugate/gen/proto => ../../gen/proto

replace github.com/costinm/hbone => ../../../hbone

replace github.com/costinm/ugate/gen/go => ../../gen/go

require (
	github.com/costinm/ugate/gen/proto v0.0.0-00010101000000-000000000000
	github.com/envoyproxy/go-control-plane v0.9.10-0.20210907150352-cf90f659a021
	github.com/golang/protobuf v1.5.2
	golang.org/x/net v0.0.0-20210825183410-e898025ed96a
	google.golang.org/grpc v1.45.0
	google.golang.org/protobuf v1.28.0
)
