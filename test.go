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
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/FTwOoO/netstack/tcpip/link/rawfile"
	"github.com/FTwOoO/netstack/tcpip/link/tun"
	"github.com/FTwOoO/netstack/tcpip/stack"
	"github.com/FTwOoO/netstack/tcpip/buffer"
	"github.com/FTwOoO/netstack/tcpip/header"
	"github.com/FTwOoO/netstack/tcpip"
	"github.com/FTwOoO/go-tun2io/tun2io"
)

var socksAddr string = "52.69.162.110:1080"
var defaultRemoteDnsServer = net.IP{8, 8, 8, 8}
var defaultDNSPort uint16 = 53
var addrName = "192.168.4.1/24"
var tunName = "tun2"

const dnsReqFre = 15 * time.Second



func main() {
	rand.Seed(time.Now().UnixNano())
	parsedIp, subnet, err := net.ParseCIDR(addrName)
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

	dialer := &tun2io.SOCKS5Dialer{SocksAddr:socksAddr}
	if err != nil {
		log.Fatal(err)
	}

	manager, err := tun2io.Tun2IO(parsedIp, subnet, linkId, true, dialer)
	if err != nil {
		log.Fatal(err)
	}

	go remoteDNSTest(parsedIp, manager.GetStack(), linkId, manager.GetNICID())
	go localDNSServerTest(parsedIp, manager.GetStack(), linkId, manager.GetNICID())
	manager.MainLoop()

}

func injectIpv4Packet(s tcpip.Stack, linkId tcpip.LinkEndpointID, nid tcpip.NICID, packetData []byte) {
	ep := stack.FindLinkEndpoint(linkId)
	if ep == nil {
		log.Fatalf("Endpoint not found:%d", linkId)
		return
	}

	d := s.(*stack.Stack).GetNic(nid)

	view := buffer.View(packetData)
	vv := buffer.NewVectorisedView(len(packetData), []buffer.View{view})
	d.DeliverNetworkPacket(ep, header.IPv4ProtocolNumber, &vv)
}

func createDNSRequst(Domain string, SrcIP net.IP, SrcPort uint16, DstIP net.IP, DstPort uint16) []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths:true}
	gopacket.SerializeLayers(buf, opts,
		&layers.IPv4{SrcIP:SrcIP, DstIP:DstIP, Protocol:layers.IPProtocolUDP},
		&layers.UDP{SrcPort:layers.UDPPort(SrcPort), DstPort:layers.UDPPort(DstPort)},
		&layers.DNS{
			ID:uint16(rand.Int31() & 0xFFFF),
			RD: true,
			OpCode:layers.DNSOpCodeQuery,
			Questions:[]layers.DNSQuestion{
				{
					Name:[]byte(Domain),
					Type:layers.DNSTypeA,
					Class:layers.DNSClassIN,
				},
			},
		},
	)

	packetData := buf.Bytes()
	return packetData
}

func remoteDNSTest(srcAddr net.IP, s tcpip.Stack, linkId tcpip.LinkEndpointID, nid tcpip.NICID) error {
	for {

		packetData := createDNSRequst("facebook.com", srcAddr, 10078, defaultRemoteDnsServer, defaultDNSPort)
		injectIpv4Packet(s, linkId, nid, packetData)

		time.Sleep(dnsReqFre)
	}

	return nil
}

func localDNSServerTest(srcAddr net.IP, s tcpip.Stack, linkId tcpip.LinkEndpointID, nid tcpip.NICID) error {
	for {

		packetData := createDNSRequst("twitter.com", srcAddr, 10079, srcAddr, defaultDNSPort)
		injectIpv4Packet(s, linkId, nid, packetData)

		time.Sleep(dnsReqFre)
	}

	return nil
}

