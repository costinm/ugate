module github.com/costinm/ugate

go 1.18

replace github.com/costinm/hbone => ../hbone

replace github.com/costinm/meshauth => ../meshauth

require (
	github.com/coreos/go-oidc v2.2.1+incompatible
	github.com/costinm/hbone v0.0.0-20221011174620-f25926c0b194
	github.com/costinm/meshauth v0.0.0-20221013185453-bb5aae6632f8
	github.com/miekg/dns v1.1.50
	golang.org/x/net v0.1.0
	golang.org/x/sys v0.1.0
)

require (
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/pquerna/cachecontrol v0.1.0 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 // indirect
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/oauth2 v0.1.0 // indirect
	golang.org/x/text v0.4.0 // indirect
	golang.org/x/tools v0.1.12 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
)
