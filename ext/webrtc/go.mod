module github.com/costinm/ugate/ext/webrtc

go 1.16

//replace github.com/costinm/ugate => ../../

require (
	github.com/costinm/ugate v0.0.0-20211023174040-9f00d2d3fca1
	github.com/pion/sctp v1.7.11
	github.com/pion/turn/v2 v2.0.5
	github.com/pion/webrtc/v3 v3.0.8
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110 // indirect
	golang.org/x/sys v0.0.0-20210220050731-9a76102bfb43 // indirect
)
