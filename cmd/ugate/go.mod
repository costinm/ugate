module github.com/costinm/ugate/cmd/ugate

go 1.16

replace github.com/costinm/ugate => ../../

replace github.com/costinm/ugate/ext/webrtc => ../../ext/webrtc

replace github.com/costinm/ugate/dns => ../../dns

replace github.com/costinm/ugate/ext/xds => ../../ext/xds

replace github.com/costinm/ugate/ext/h2r => ../../ext/h2r

replace github.com/costinm/ugate/ext/quic => ../../ext/quic

replace github.com/lucas-clemente/quic-go => ../../../quic

require (
	github.com/costinm/ugate v0.0.0-20210419001517-08ea89abf527
	github.com/costinm/ugate/dns v0.0.0-00010101000000-000000000000
	github.com/costinm/ugate/ext/h2r v0.0.0-00010101000000-000000000000
	github.com/costinm/ugate/ext/quic v0.0.0-00010101000000-000000000000
	github.com/costinm/ugate/ext/webrtc v0.0.0-00010101000000-000000000000
	github.com/costinm/ugate/ext/xds v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.36.1
)
