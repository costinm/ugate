package main

import (
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/server6"
)

import (
	"log"
)

func handler(conn net.PacketConn, peer net.Addr, m dhcpv6.DHCPv6) {
	// this function will just print the received DHCPv6 message, without replying
	log.Print(m.Summary())
}
func handler4(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	// this function will just print the received DHCPv6 message, without replying
	log.Print(m.Summary())
}

func main() {
	if false {
		laddr := &net.UDPAddr{
			IP:   net.ParseIP("::1"),
			Port: dhcpv6.DefaultServerPort,
		}
		server, err := server6.NewServer("", laddr, handler)
		if err != nil {
			log.Fatal(err)
		}

		go server.Serve()
	}

	laddr4 := &net.UDPAddr{
		Port: dhcpv4.ServerPort,
	}
	s4, err := server4.NewServer("", laddr4, handler4)
	if err != nil {
		log.Fatal(err)
	}
	s4.Serve()
}
