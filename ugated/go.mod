module github.com/costinm/ugate/ugated

go 1.21

require github.com/mochi-co/mqtt v1.3.2

replace github.com/costinm/ugate => ../

replace github.com/costinm/ssh-mesh => ../../ssh-mesh
replace github.com/costinm/ugate/pkg/ext/lwip => ../pkg/ext/lwip

replace github.com/costinm/utel => ../../utel

//require github.com/quic-go/qtls-go1-20 v0.2.2

replace github.com/costinm/meshauth => ../../meshauth

//Larger buffer, hooks to use the h3 stack
//replace github.com/lucas-clemente/quic-go => ../../../quic
//replace github.com/lucas-clemente/quic-go => github.com/costinm/quic v0.5.1-0.20210425224043-9f67435d0255

replace github.com/eycorsican/go-tun2socks => github.com/costinm/go-tun2socks v1.16.12-0.20210328172757-88f6d54235cb

require (
	github.com/costinm/meshauth v0.0.0-20240105003042-ccb7c7765ee0
	github.com/costinm/ugate v0.0.0-20221024013023-789def6d5dde
	github.com/golang/protobuf v1.5.3
	github.com/gorilla/websocket v1.5.1
	github.com/pion/sctp v1.8.9
	github.com/pion/turn/v2 v2.1.4
	github.com/pion/webrtc/v3 v3.2.24
	golang.org/x/exp v0.0.0-20240103183307-be819d1f06fc

)

require (
	github.com/miekg/dns v1.1.57 // indirect
	golang.org/x/net v0.19.0
	golang.org/x/sys v0.16.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

require (
	github.com/costinm/ssh-mesh v0.0.0-20240101190630-66786111a72d // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/uuid v1.5.0 // indirect
	github.com/pion/datachannel v1.5.5 // indirect
	github.com/pion/dtls/v2 v2.2.9 // indirect
	github.com/pion/ice/v2 v2.3.11 // indirect
	github.com/pion/interceptor v0.1.25 // indirect
	github.com/pion/logging v0.2.2 // indirect
	github.com/pion/mdns v0.0.9 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.13 // indirect
	github.com/pion/rtp v1.8.3 // indirect
	github.com/pion/sdp/v3 v3.0.6 // indirect
	github.com/pion/srtp/v2 v2.0.18 // indirect
	github.com/pion/stun v0.6.1 // indirect
	github.com/pion/transport v0.14.1 // indirect
	github.com/pion/transport/v2 v2.2.4 // indirect
	github.com/pion/udp v0.1.4 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/xid v1.5.0 // indirect
	github.com/stretchr/testify v1.8.4 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/tools v0.16.1 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
