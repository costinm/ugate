module github.com/costinm/ugate/ugated

go 1.19

require github.com/mochi-co/mqtt v1.3.2

replace github.com/costinm/ugate => ../

replace github.com/costinm/ssh-mesh => ../../ssh-mesh

replace github.com/costinm/hbone => ../../hbone

replace github.com/costinm/meshauth => ../../meshauth

//Larger buffer, hooks to use the h3 stack
//replace github.com/lucas-clemente/quic-go => ../../../quic
//replace github.com/lucas-clemente/quic-go => github.com/costinm/quic v0.5.1-0.20210425224043-9f67435d0255

replace github.com/eycorsican/go-tun2socks => github.com/costinm/go-tun2socks v1.16.12-0.20210328172757-88f6d54235cb

require (
	github.com/costinm/hbone v0.0.0-20221011174620-f25926c0b194
	github.com/costinm/meshauth v0.0.0-20221013185453-bb5aae6632f8
	github.com/costinm/ssh-mesh v0.0.0-20221014163420-3421d7ade346
	github.com/costinm/ugate v0.0.0-20221014040536-984a9753d81c
	github.com/eycorsican/go-tun2socks v1.16.12-0.20201107203946-301549c435ff
	github.com/gorilla/websocket v1.5.0
	github.com/lucas-clemente/quic-go v0.29.2
	github.com/pion/sctp v1.8.3
	github.com/pion/turn/v2 v2.0.8
	github.com/pion/webrtc/v3 v3.1.47
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8
	golang.org/x/crypto v0.0.0-20221012134737-56aed061732a
	gvisor.dev/gvisor v0.0.0-20221019184736-ae7cca128546
)

require (
	github.com/creack/pty v1.1.18 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/miekg/dns v1.1.50 // indirect
	github.com/pkg/sftp v1.13.5 // indirect
	golang.org/x/net v0.1.0
	golang.org/x/sys v0.1.0 // indirect
	golang.org/x/text v0.4.0 // indirect
)

require (
	github.com/bazelbuild/rules_go v0.30.0 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-task/slim-sprig v0.0.0-20210107165309-348f09dbbbc0 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/marten-seemann/qpack v0.2.1 // indirect
	github.com/marten-seemann/qtls-go1-18 v0.1.3 // indirect
	github.com/marten-seemann/qtls-go1-19 v0.1.1 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/pion/datachannel v1.5.2 // indirect
	github.com/pion/dtls/v2 v2.1.5 // indirect
	github.com/pion/ice/v2 v2.2.11 // indirect
	github.com/pion/interceptor v0.1.11 // indirect
	github.com/pion/logging v0.2.2 // indirect
	github.com/pion/mdns v0.0.5 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.10 // indirect
	github.com/pion/rtp v1.7.13 // indirect
	github.com/pion/sdp/v3 v3.0.6 // indirect
	github.com/pion/srtp/v2 v2.0.10 // indirect
	github.com/pion/stun v0.3.5 // indirect
	github.com/pion/transport v0.13.1 // indirect
	github.com/pion/udp v0.1.1 // indirect
	github.com/rs/xid v1.4.0 // indirect
	golang.org/x/exp v0.0.0-20220722155223-a9213eeb770e // indirect
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	golang.org/x/tools v0.1.12 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
)
