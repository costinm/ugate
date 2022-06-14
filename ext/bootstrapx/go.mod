module github.com/costinm/ugate/cmd/ugate

go 1.18

replace github.com/costinm/ugate => ../../

replace github.com/costinm/ugate/ext/gvisor => ../gvisor

replace github.com/costinm/ugate/ext/lwip => ../lwip

replace gvisor.dev/gvisor => github.com/costinm/gvisor v0.0.0-20210509154143-a94fe58cda62

replace github.com/eycorsican/go-tun2socks => github.com/costinm/go-tun2socks v1.16.12-0.20210328172757-88f6d54235cb

replace github.com/costinm/ugate/ext/webrtc => ../webrtc

replace github.com/costinm/ugate/ext/xds => ../xds

require (
	github.com/costinm/ugate v0.0.0-20211023174040-9f00d2d3fca1
	github.com/costinm/ugate/ext/gvisor v0.0.0-20210509234022-4f213a5560be
	github.com/costinm/ugate/ext/lwip v0.0.0-20210509234022-4f213a5560be
	github.com/costinm/ugate/ext/webrtc v0.0.0-20210425213441-05024f5e8910
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/pion/ice/v2 v2.1.6 // indirect
	github.com/pion/rtp v1.6.5 // indirect
	github.com/pion/webrtc/v3 v3.0.25 // indirect
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8
	golang.org/x/net v0.0.0-20210423184538-5f58ad60dda6 // indirect
	golang.org/x/sys v0.0.0-20210423185535-09eb48e85fd7 // indirect
)
