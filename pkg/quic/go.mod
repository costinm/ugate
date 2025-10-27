module github.com/costinm/ugate/pkg/quic

go 1.23

toolchain go1.23.4

replace github.com/costinm/ugate => ../..

require (
	github.com/costinm/meshauth v0.0.0-20240803190121-2a6dfc0e888a
	github.com/costinm/ssh-mesh v0.0.0-20240229060027-2867ec3f4a46
	github.com/costinm/ugate v0.0.0-00010101000000-000000000000
	github.com/quic-go/quic-go v0.48.2
	github.com/quic-go/webtransport-go v0.8.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/pprof v0.0.0-20240727154555-813a5fbdbec8 // indirect
	github.com/onsi/ginkgo/v2 v2.19.1 // indirect
	github.com/onsi/gomega v1.34.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/quic-go/qpack v0.5.1 // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	go.uber.org/mock v0.4.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/exp v0.0.0-20240904232852-e7e105dedf7e // indirect
	golang.org/x/mod v0.22.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/tools v0.27.0 // indirect
)
