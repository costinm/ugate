module github.com/costinm/ugate/ext/bootstrap

go 1.18

replace github.com/costinm/ugate => ../../

replace github.com/costinm/ugate/dns => ../../dns

replace github.com/costinm/ugate/ext/h2r => ../h2r

replace github.com/costinm/ugate/ext/quic => ../quic

replace github.com/costinm/ugate/ext/xds => ../xds

replace github.com/costinm/ssh-mesh => ../../../ssh-mesh

//Larger buffer, hooks to use the h3 stack
//replace github.com/lucas-clemente/quic-go => ../../../quic
//replace github.com/lucas-clemente/quic-go => github.com/costinm/quic v0.5.1-0.20210425224043-9f67435d0255

require (
	github.com/costinm/ssh-mesh v0.0.0-20220429182219-8b008c6822f6
	github.com/costinm/ugate v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/dns v0.0.0-20211023174040-9f00d2d3fca1
	github.com/costinm/ugate/ext/h2r v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/quic v0.0.0-20210425213441-05024f5e8910
	github.com/gorilla/websocket v1.5.0
	golang.org/x/crypto v0.0.0-20210503195802-e9a32991a82e
)

require (
	github.com/cheekybits/genny v1.0.0 // indirect
	github.com/creack/pty v1.1.13 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/lucas-clemente/quic-go v0.20.1 // indirect
	github.com/marten-seemann/qpack v0.2.1 // indirect
	github.com/marten-seemann/qtls-go1-15 v0.1.4 // indirect
	github.com/marten-seemann/qtls-go1-16 v0.1.3 // indirect
	github.com/miekg/dns v1.1.40 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/sftp v1.13.1 // indirect
	golang.org/x/net v0.0.0-20210423184538-5f58ad60dda6 // indirect
	golang.org/x/sys v0.0.0-20210603081109-ebe580a85c40 // indirect
	golang.org/x/text v0.3.6 // indirect
)
