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
	github.com/costinm/ugate v0.0.0-20220614135442-cafcfb6d0da4
	github.com/costinm/ugate/dns v0.0.0-20211023174040-9f00d2d3fca1
	github.com/costinm/ugate/ext/h2r v0.0.0-20210425213441-05024f5e8910
	github.com/gorilla/websocket v1.5.0
	golang.org/x/crypto v0.0.0-20210503195802-e9a32991a82e
)

require (
	github.com/costinm/hbone v0.0.0-20220731143958-835b4d46903e // indirect
	github.com/creack/pty v1.1.13 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/miekg/dns v1.1.40 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/sftp v1.13.1 // indirect
	golang.org/x/net v0.0.0-20211014172544-2b766c08f1c0 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/sys v0.0.0-20210831042530-f4d43177bf5e // indirect
	golang.org/x/text v0.3.7 // indirect
)
