module github.com/costinm/ugate/ext/xds

go 1.16

replace github.com/costinm/ugate => ../../

require (
	github.com/costinm/ugate v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/dns v0.0.0-20211023174040-9f00d2d3fca1 // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	golang.org/x/net v0.0.0-20210423184538-5f58ad60dda6
	golang.org/x/sys v0.0.0-20210423185535-09eb48e85fd7 // indirect
	google.golang.org/genproto v0.0.0-20210423144448-3a41ef94ed2b // indirect
	google.golang.org/grpc v1.37.0
	google.golang.org/protobuf v1.26.0
)
