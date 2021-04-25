module github.com/costinm/ugate/cmd/ugate

go 1.16

//replace github.com/costinm/ugate => ../../

//replace github.com/costinm/ugate/ext/webrtc => ../../ext/webrtc

//replace github.com/costinm/ugate/dns => ../../dns
//
//replace github.com/costinm/ugate/ext/xds => ../../ext/xds
//
//replace github.com/costinm/ugate/ext/h2r => ../../ext/h2r
//
//replace github.com/costinm/ugate/ext/quic => ../../ext/quic

//replace github.com/lucas-clemente/quic-go => ../../../quic

replace github.com/lucas-clemente/quic-go => github.com/costinm/quic v0.5.1-0.20210425224043-9f67435d0255

require (
	github.com/costinm/ugate v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/dns v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/h2r v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/quic v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/webrtc v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/xds v0.0.0-20210425213441-05024f5e8910
	github.com/lucas-clemente/quic-go v0.20.1 // indirect
	github.com/miekg/dns v1.1.41 // indirect
	github.com/pion/ice/v2 v2.1.6 // indirect
	github.com/pion/rtp v1.6.5 // indirect
	github.com/pion/webrtc/v3 v3.0.25 // indirect
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b // indirect
	golang.org/x/net v0.0.0-20210423184538-5f58ad60dda6 // indirect
	golang.org/x/sys v0.0.0-20210423185535-09eb48e85fd7 // indirect
	google.golang.org/genproto v0.0.0-20210423144448-3a41ef94ed2b // indirect
	google.golang.org/grpc v1.37.0
)
