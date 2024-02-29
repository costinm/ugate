module github.com/costinm/ugate/cmd

go 1.21

replace github.com/costinm/ugate => ../

replace github.com/costinm/ssg-mesh => ../../ssh-mesh

require (
	github.com/costinm/ssh-mesh v0.0.0-20240101190630-66786111a72d // indirect
	github.com/eycorsican/go-tun2socks v1.16.11 // indirect
	github.com/songgao/water v0.0.0-20190725173103-fd331bda3f4b // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/exp v0.0.0-20231219180239-dc181d75b848 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
)
