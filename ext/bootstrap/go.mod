module github.com/costinm/ugate/ext/bootstrap

go 1.16

replace github.com/costinm/ugate => ../../

replace github.com/costinm/ugate/dns => ../../dns

replace github.com/costinm/ugate/ext/h2r => ../h2r

replace github.com/costinm/ugate/ext/quic => ../quic
replace github.com/costinm/ugate/ext/xds => ../xds

//Larger buffer, hooks to use the h3 stack
//replace github.com/lucas-clemente/quic-go => ../../../quic
//replace github.com/lucas-clemente/quic-go => github.com/costinm/quic v0.5.1-0.20210425224043-9f67435d0255

require (
	github.com/costinm/ugate v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/h2r v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/quic v0.0.0-20210425213441-05024f5e8910
	github.com/costinm/ugate/ext/ssh v0.0.0-20210617045128-ebd612515d2b
	github.com/costinm/ugate/ext/xds v0.0.0-20210617045128-ebd612515d2b
	google.golang.org/grpc v1.37.0
)
