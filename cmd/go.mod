module github.com/costinm/ugate/cmd

go 1.18

replace github.com/costinm/ugate => ../

replace github.com/costinm/meshauth => ../../meshauth

replace github.com/costinm/hbone => ../../hbone

replace github.com/costinm/hbone/hboned => ../../hbone/hboned

require github.com/costinm/ugate v0.0.0-20220802234414-af1ce48b30c0

require (
	github.com/costinm/meshauth v0.0.0-20221013185453-bb5aae6632f8 // indirect
	golang.org/x/net v0.0.0-20221012135044-0b7e1fb9d458 // indirect
	golang.org/x/sys v0.0.0-20221013171732-95e765b1cc43 // indirect
	golang.org/x/text v0.3.8 // indirect
)
