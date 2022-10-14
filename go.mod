module github.com/costinm/ugate

go 1.18

replace github.com/costinm/hbone => ../hbone

replace github.com/costinm/meshauth => ../meshauth

require (
	github.com/costinm/meshauth v0.0.0-20221013185453-bb5aae6632f8
	github.com/costinm/ugate/test v0.0.0-20220802234414-af1ce48b30c0
	golang.org/x/net v0.0.0-20221012135044-0b7e1fb9d458
	golang.org/x/sys v0.0.0-20221013171732-95e765b1cc43
)

require golang.org/x/text v0.3.8 // indirect
