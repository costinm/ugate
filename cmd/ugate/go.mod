module github.com/costinm/ugate/cmd/ugate

go 1.19

replace github.com/costinm/ugate => ../../

replace github.com/costinm/ugate/dns => ../../dns

replace github.com/costinm/ugate/test => ../../test

replace github.com/costinm/ugate/ext/bootstrap => ../../ext/bootstrap

replace github.com/costinm/ugate/ext/bootstrapx => ../../ext/bootstrapx

replace github.com/costinm/ugate/ext/quic => ../../ext/quic

replace github.com/costinm/ugate/ext/h2r => ../../ext/h2r

replace github.com/costinm/ugate/ext/ssh => ../../ext/ssh

replace github.com/costinm/ssh-mesh => ../../../ssh-mesh

replace github.com/costinm/meshauth => ../../../meshauth

require (
	github.com/costinm/ugate v0.0.0-20220614135442-cafcfb6d0da4
	github.com/costinm/ugate/ext/bootstrap v0.0.0-20210510001934-3cec7b4617c7
	github.com/miekg/dns v1.1.41 // indirect
)

require github.com/costinm/ugate/test v0.0.0-20220802234414-af1ce48b30c0

require (
	github.com/costinm/meshauth v0.0.0-20221013185453-bb5aae6632f8 // indirect
	github.com/costinm/ssh-mesh v0.0.0-20220429182219-8b008c6822f6 // indirect
	github.com/costinm/ugate/dns v0.0.0-20211023174040-9f00d2d3fca1 // indirect
	github.com/costinm/ugate/ext/h2r v0.0.0-20210425213441-05024f5e8910 // indirect
	github.com/creack/pty v1.1.13 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/sftp v1.13.1 // indirect
	golang.org/x/crypto v0.0.0-20210503195802-e9a32991a82e // indirect
	golang.org/x/net v0.0.0-20211014172544-2b766c08f1c0 // indirect
	golang.org/x/sys v0.0.0-20220926163933-8cfa568d3c25 // indirect
	golang.org/x/text v0.3.7 // indirect
)
