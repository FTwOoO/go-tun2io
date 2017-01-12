/*
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 * Author: FTwOoO <booobooob@gmail.com>
 */
package main

import (
	"log"
	"math/rand"
	"net"
	"time"
	"./tun2io"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/FTwOoO/netstack/tcpip/link/rawfile"
	"github.com/FTwOoO/netstack/tcpip/link/tun"
	"github.com/FTwOoO/netstack/tcpip/stack"
	"github.com/FTwOoO/netstack/tcpip/buffer"
	"github.com/FTwOoO/netstack/tcpip/header"
	"github.com/FTwOoO/netstack/tcpip"
	"github.com/FTwOoO/netstack/tcpip/network/ipv4"
	"github.com/FTwOoO/dnsrelay/dnsrelay"
	"github.com/FTwOoO/vpncore/net/addr"
)

var socksAddr string = "52.69.162.110:1080"
var defaultRemoteDnsServer = net.IP{8, 8, 8, 8}
var addrName = "192.168.4.1/24"
var tunName = "tun2"

const dnsReqFre = 15 * time.Second

func main() {
	rand.Seed(time.Now().UnixNano())
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

	linkId, err := tun2io.CreateFdLinkEndpoint(fd, mtu)
	if err != nil {
		log.Fatal(err)
	}

	s, err := tun2io.CreateStack(parsedAddr, 1, linkId)
	if err != nil {
		log.Fatal(err)
	}

	ep, err := tun2io.CreateUdpEndpoint(s, ipv4.ProtocolNumber, tcpip.FullAddress{NIC:1, Addr:tcpip.Address(parsedAddr.To4()), Port:53})
	if err != nil {
		log.Fatal(err)
	}

	handlerServ, _ := dnsrelay.NewDNSServer(
		&dnsrelay.Config{
			DefaultGroups:[]string{"serv"},
			DNSCache:dnsrelay.DNSCache{Backend:"memory", Expire:3600, Maxcount:500},
			DNSGroups:map[string][]addr.DNSAddresss{"serv":[]addr.DNSAddresss{{Ip:defaultRemoteDnsServer, Port:53}}},
		}, true)

	_, err = tun2io.CreateDnsServer(ep, handlerServ)
	if err != nil {
		log.Fatal(err)
	}

	dialer := &tun2io.SOCKS5Dialer{SocksAddr:socksAddr}
	if err != nil {
		log.Fatal(err)
	}

	//go remoteDNSTest(parsedAddr, s, linkId, 1)
	go localDNSServerTest(parsedAddr, s, linkId, 1)

	manager, err := tun2io.NewTun2ioManager(s, dialer)
	manager.MainLoop()
}

func remoteDNSTest(srcAddr net.IP, s tcpip.Stack, linkId tcpip.LinkEndpointID, nid tcpip.NICID) error {
	for {
		ep := stack.FindLinkEndpoint(linkId)
		if ep == nil {
			log.Fatalf("Endpoint not found:%d", linkId)
			return tcpip.ErrBadLinkEndpoint
		}

		d := s.(*stack.Stack).GetNic(nid)

		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{FixLengths:true}
		gopacket.SerializeLayers(buf, opts,
			&layers.IPv4{SrcIP:srcAddr, DstIP:defaultRemoteDnsServer, Protocol:layers.IPProtocolUDP},
			&layers.UDP{SrcPort:10078, DstPort:53},
			&layers.DNS{
				ID:uint16(rand.Int31() & 0xFFFF),
				RD: true,
				OpCode:layers.DNSOpCodeQuery,
				Questions:[]layers.DNSQuestion{
					{
						Name:[]byte("xahlee.info"),
						Type:layers.DNSTypeA,
						Class:layers.DNSClassIN,
					},
				},
			},
		)

		packetData := buf.Bytes()
		view := buffer.View(packetData)
		vv := buffer.NewVectorisedView(len(packetData), []buffer.View{view})
		d.DeliverNetworkPacket(ep, header.IPv4ProtocolNumber, &vv)

		time.Sleep(dnsReqFre)
	}

	return nil
}

func localDNSServerTest(srcAddr net.IP, s tcpip.Stack, linkId tcpip.LinkEndpointID, nid tcpip.NICID) error {
	for {
		ep := stack.FindLinkEndpoint(linkId)
		if ep == nil {
			log.Fatalf("Endpoint not found:%d", linkId)
			return tcpip.ErrBadLinkEndpoint
		}

		d := s.(*stack.Stack).GetNic(nid)

		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{FixLengths:true}
		gopacket.SerializeLayers(buf, opts,
			&layers.IPv4{SrcIP:srcAddr, DstIP:srcAddr, Protocol:layers.IPProtocolUDP},
			&layers.UDP{SrcPort:10078, DstPort:53},
			&layers.DNS{
				ID:uint16(rand.Int31() & 0xFFFF),
				RD: true,
				OpCode:layers.DNSOpCodeQuery,
				Questions:[]layers.DNSQuestion{
					{
						Name:[]byte("twitter.com"),
						Type:layers.DNSTypeA,
						Class:layers.DNSClassIN,
					},
				},
			},
		)

		packetData := buf.Bytes()
		view := buffer.View(packetData)
		vv := buffer.NewVectorisedView(len(packetData), []buffer.View{view})
		d.DeliverNetworkPacket(ep, header.IPv4ProtocolNumber, &vv)

		time.Sleep(dnsReqFre)
	}

	return nil
}

