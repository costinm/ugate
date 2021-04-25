module github.com/costinm/ugate/ext/quic

go 1.16

replace github.com/costinm/ugate => ../../

replace github.com/lucas-clemente/quic-go => ../../../quic
//replace github.com/lucas-clemente/quic-go => github.com/costinm/quic v0.5.1-0.20210425150008-d8e6379c24ed

require (
	github.com/costinm/ugate v0.0.0-20210221155556-10edd21fadbf
	github.com/lucas-clemente/quic-go v0.7.1-0.20210421053939-84e03e59760c
	github.com/marten-seemann/qpack v0.2.1
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110 // indirect
	golang.org/x/sys v0.0.0-20210220050731-9a76102bfb43 // indirect
)
