module github.com/costinm/ugate/ext/lwip

go 1.16

//replace github.com/costinm/ugate => ../..

//replace github.com/eycorsican/go-tun2socks => ../../go-tun2socks

replace github.com/eycorsican/go-tun2socks => github.com/costinm/go-tun2socks v1.16.12-0.20210328172757-88f6d54235cb

require (
	github.com/eycorsican/go-tun2socks v1.16.12-0.20201107203946-301549c435ff
	github.com/songgao/water v0.0.0-20190725173103-fd331bda3f4b
	golang.org/x/sys v0.0.0-20201119102817-f84b799fce68 // indirect
)
