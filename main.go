// Copyright 2016 The Netstack Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This sample creates a stack with TCP and IPv4 protocols on top of a TUN
// device, and listens on a port. Data received by the server in the accepted
// connections is echoed back to the clients.
package main

import (
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	"github.com/FTwOoO/netstack/tcpip"
	"github.com/FTwOoO/netstack/tcpip/link/fdbased"
	"github.com/FTwOoO/netstack/tcpip/link/rawfile"
	"github.com/FTwOoO/netstack/tcpip/link/tun"
	"github.com/FTwOoO/netstack/tcpip/network/ipv4"
	"github.com/FTwOoO/netstack/tcpip/network/ipv6"
	"github.com/FTwOoO/netstack/tcpip/stack"
	"github.com/FTwOoO/netstack/tcpip/transport/tcp"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("Usage: ", os.Args[0], " <tun-device> <local-address>")
	}

	tunName := os.Args[1]
	addrName := os.Args[2]

	rand.Seed(time.Now().UnixNano())

	// Parse the IP address. Support both ipv4 and ipv6.
	parsedAddr, _, err := net.ParseCIDR(addrName)
	if err != nil {
		log.Fatalf("Bad IP address: %v", addrName)
	}

	mtu, err := rawfile.GetMTU(tunName)
	if err != nil {
		log.Fatal(err)
	}

	fd, err := tun.Open(tunName)
	if err != nil {
		log.Fatal(err)
	}

	_ = CreateStackWithFd(parsedAddr, fd, mtu)

	for {
		time.Sleep(5 * time.Second)
	}

}

func CreateStackWithFd(mainAddr net.IP, fd int, mtu int) tcpip.Stack {
	var addr tcpip.Address
	var proto tcpip.NetworkProtocolNumber

	if mainAddr.To4() != nil {
		addr = tcpip.Address(mainAddr.To4())
		proto = ipv4.ProtocolNumber
	} else if mainAddr.To16() != nil {
		addr = tcpip.Address(mainAddr.To16())
		proto = ipv6.ProtocolNumber
	} else {
		log.Fatalf("Unknown IP type: %v", mainAddr)
	}


	// Create the stack with ip and tcp protocols, then add a tun-based
	// NIC and address.
	s := stack.New([]string{ipv4.ProtocolName, ipv6.ProtocolName}, []string{tcp.ProtocolName})

	linkID := fdbased.New(fd, mtu, nil)
	if err := s.CreateNIC(1, linkID); err != nil {
		log.Fatal(err)
	}

	s.SetForwardMode(true)

	if err := s.AddAddress(1, proto, addr); err != nil {
		log.Fatal(err)
	}

	// Add default route.
	s.SetRouteTable([]tcpip.Route{
		{
			Destination: tcpip.Address(strings.Repeat("\x00", len(addr))),
			Mask:        tcpip.Address(strings.Repeat("\x00", len(addr))),
			Gateway:     "",
			NIC:         1,
		},
	})

	return s
}
