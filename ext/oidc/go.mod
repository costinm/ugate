module github.com/costnm/ugate/ext/oidc

go 1.16

replace (
	github.com/costinm/ugate => ../../
	github.com/costinm/ugate/auth => ../../auth/
)

require (
	github.com/coreos/go-oidc v2.2.1+incompatible
	github.com/pquerna/cachecontrol v0.1.0 // indirect
	golang.org/x/oauth2 v0.0.0-20210622215436-a8dc77f794b6 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
)
