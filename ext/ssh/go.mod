module github.com/costinm/ugate/ext/ssh

go 1.16

replace github.com/costinm/ugate => ../../

require (
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	github.com/costinm/ugate v0.0.0-00010101000000-000000000000
	github.com/creack/pty v1.1.13
	github.com/gliderlabs/ssh v0.3.2
	github.com/google/uuid v1.2.0
	github.com/pkg/sftp v1.13.1
	golang.org/x/crypto v0.0.0-20210503195802-e9a32991a82e
)
