package main

import (
	"context"
	"net"

	"github.com/costinm/ugate/pkg/smtpd"
)

/*
go build -buildmode=plugin -o /tmp/smtpd.so .

-> 12M binary
- libc, ld, vdso

 */


func New() any {
	return smtpd.New()
}

func main() {
	sd := smtpd.New()

	var err error
	sd.NetListener, err = net.Listen("tcp", "1025")
	if err != nil {
		panic(err)
	}

	sd.Start(context.Background())

	select{}
}
