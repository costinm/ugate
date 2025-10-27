module github.com/costinm/ugate/ugatex/pkg/cfquiche

go 1.21

replace github.com/costinm/ugate => ./../../..

replace github.com/costinm/ugate/ugated => ./../..

replace github.com/costinm/ugate/ugatex => ./../../../ugatex

replace github.com/costinm/meshauth => ./../../../../meshauth

require (
	github.com/costinm/hbone v0.0.0-20221011174620-f25926c0b194
	github.com/costinm/ugate v0.0.0-20221014040536-984a9753d81c
	github.com/costinm/ugate/auth v0.0.0-20220802234414-af1ce48b30c0 // indirect
	github.com/costinm/ugate/ugated v0.0.0-00010101000000-000000000000 // indirect
	github.com/goburrow/quiche v0.0.0-20190827130453-f5320f3ec7dd
)
