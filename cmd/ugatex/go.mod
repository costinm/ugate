module github.com/costinm/ugate/cmd/ugate

go 1.16

replace github.com/costinm/ugate => ../../

replace github.com/costinm/ugate/ext/webrtc => ../../ext/webrtc

replace github.com/costinm/ugate/dns => ../../dns

replace github.com/costinm/ugate/ext/bootstrap => ../../ext/bootstrap

replace github.com/costinm/ugate/ext/bootstrapx => ../../ext/bootstrapx

replace github.com/costinm/ugate/ext/xds => ../../ext/xds

replace github.com/costinm/ugate/ext/h2r => ../../ext/h2r

replace github.com/costinm/ugate/ext/quic => ../../ext/quic

replace github.com/lucas-clemente/quic-go => ../../../quic

replace github.com/costinm/ugate/ext/gvisor => ../../ext/gvisor
replace github.com/costinm/ugate/ext/lwip => ../../ext/lwip

//replace gvisor.dev/gvisor => github.com/costinm/gvisor v0.0.0-20210509154143-a94fe58cda62
replace gvisor.dev/gvisor => ../../../gvisor

replace github.com/eycorsican/go-tun2socks => github.com/costinm/go-tun2socks v1.16.12-0.20210328172757-88f6d54235cb

//replace github.com/lucas-clemente/quic-go => github.com/costinm/quic v0.5.1-0.20210425224043-9f67435d0255

require (
	github.com/costinm/ugate v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/dns v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/bootstrap v0.0.0-00010101000000-000000000000
	github.com/costinm/ugate/ext/bootstrapx v0.0.0-00010101000000-000000000000
)
