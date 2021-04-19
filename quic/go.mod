module github.com/costinm/ugate/quic

go 1.16

replace github.com/costinm/ugate => ../

require (
	github.com/costinm/ugate v0.0.0-20210221155556-10edd21fadbf
	github.com/lucas-clemente/quic-go v0.20.1
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110 // indirect
	golang.org/x/sys v0.0.0-20210220050731-9a76102bfb43 // indirect
)
