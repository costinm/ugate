module github.com/costinm/ugate/cmd/ugate

go 1.16

replace github.com/costinm/ugate => ../../
replace github.com/costinm/ugate/webrtc => ../../webrtc
replace github.com/costinm/ugate/dns => ../../dns
replace github.com/costinm/ugate/webpush => ../../webpush

require (
	github.com/costinm/ugate v0.0.0-20210221155556-10edd21fadbf

	golang.org/x/net v0.0.0-20201224014010-6772e930b67b
	golang.org/x/sys v0.0.0-20201119102817-f84b799fce68
)
