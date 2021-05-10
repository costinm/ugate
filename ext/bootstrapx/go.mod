module github.com/costinm/ugate/cmd/ugate

go 1.16

replace github.com/costinm/ugate => ../../

replace github.com/costinm/ugate/ext/webrtc => ../../ext/webrtc

replace github.com/costinm/ugate/dns => ../../dns

replace github.com/costinm/ugate/ext/xds => ../../ext/xds

replace github.com/costinm/ugate/ext/h2r => ../../ext/h2r

//replace github.com/costinm/ugate/ext/gvisor => ../../ext/gvisor

//replace github.com/costinm/ugate/ext/lwip => ../../ext/lwip

replace github.com/costinm/ugate/ext/quic => ../../ext/quic

replace gvisor.dev/gvisor => github.com/costinm/gvisor v0.0.0-20210509154143-a94fe58cda62

replace github.com/eycorsican/go-tun2socks => github.com/costinm/go-tun2socks v1.16.12-0.20210328172757-88f6d54235cb

//replace github.com/lucas-clemente/quic-go => github.com/costinm/quic v0.5.1-0.20210425224043-9f67435d0255

require (
	github.com/costinm/ugate v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/gvisor v0.0.0-20210509234022-4f213a5560be
	github.com/costinm/ugate/ext/lwip v0.0.0-20210509234022-4f213a5560be
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8
	golang.org/x/net v0.0.0-20210423184538-5f58ad60dda6 // indirect
	golang.org/x/sys v0.0.0-20210423185535-09eb48e85fd7 // indirect
)
