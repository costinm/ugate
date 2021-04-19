module github.com/costinm/ugate/cmd/ugate

go 1.16

replace github.com/costinm/ugate => ../../

replace github.com/costinm/ugate/webrtc => ../../webrtc

replace github.com/costinm/ugate/dns => ../../dns

replace github.com/costinm/ugate/xds => ../../xds

replace github.com/costinm/ugate/quic => ../../quic

require (
	github.com/costinm/ugate v0.0.0-20210328173325-afc113d007e8
	github.com/costinm/ugate/dns v0.0.0-00010101000000-000000000000
	github.com/costinm/ugate/quic v0.0.0-00010101000000-000000000000
	github.com/costinm/ugate/webrtc v0.0.0-00010101000000-000000000000
	github.com/costinm/ugate/xds v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.36.1
)
