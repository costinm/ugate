module github.com/costinm/ugate/cmd/ugate

go 1.16

replace github.com/costinm/ugate => ../../

replace github.com/costinm/ugate/dns => ../../dns

replace github.com/costinm/ugate/ext/bootstrap => ../../ext/bootstrap

replace github.com/costinm/ugate/ext/quic => ../../ext/quic

replace github.com/costinm/ugate/ext/ssh => ../../ext/ssh

require (
	github.com/costinm/ugate v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/dns v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/bootstrap v0.0.0-20210510001934-3cec7b4617c7
	github.com/miekg/dns v1.1.41 // indirect
)
