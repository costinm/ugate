module github.com/costinm/ugate

go 1.18

replace (
	github.com/costinm/hbone => ../hbone
	github.com/costinm/ugate/auth => ./auth
)

require (
	golang.org/x/net v0.0.0-20211014172544-2b766c08f1c0
	golang.org/x/sys v0.0.0-20210423082822-04245dca01da
)

require (
	github.com/costinm/hbone v0.0.0-20220628165743-43be365c5ba8 // indirect
	github.com/costinm/ugate/auth v0.0.0-00010101000000-000000000000 // indirect
)
