module github.com/costinm/ugate/cmd/ugate

go 1.16

replace github.com/costinm/ugate => ../../

replace github.com/costinm/ugate/webrtc => ../../webrtc

replace github.com/costinm/ugate/dns => ../../dns

replace github.com/costinm/ugate/webpush => ../../webpush

require (
	github.com/costinm/ugate v0.0.0-20210328173325-afc113d007e8
	github.com/costinm/ugate/dns v0.0.0-00010101000000-000000000000
	github.com/costinm/ugate/webpush v0.0.0-20210329161419-fd5474ea74fe
)
