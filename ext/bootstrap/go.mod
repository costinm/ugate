module github.com/costinm/ugate/ext/bootstrap

go 1.16

replace github.com/costinm/ugate => ../../

replace github.com/costinm/ugate/dns => ../../dns

replace github.com/costinm/ugate/ext/h2r => ../h2r

replace github.com/costinm/ugate/ext/quic => ../quic

//Larger buffer, hooks to use the h3 stack
//replace github.com/lucas-clemente/quic-go => ../../../quic
//replace github.com/lucas-clemente/quic-go => github.com/costinm/quic v0.5.1-0.20210425224043-9f67435d0255

require (
	github.com/costinm/ugate v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/h2r v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/quic v0.0.0-20210425213441-05024f5e8910
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b // indirect
)
