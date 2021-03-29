module github.com/costinm/ugate/xds

go 1.16

replace github.com/costinm/ugate => ../

require (
	github.com/costinm/ugate v0.0.0-20210328173325-afc113d007e8
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.1
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	google.golang.org/grpc v1.36.1
	google.golang.org/protobuf v1.26.0
)
