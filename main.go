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
	"os"
	"time"
	"github.com/FTwOoO/netstack/tcpip/link/rawfile"
	"github.com/FTwOoO/netstack/tcpip/link/tun"
	"./tun2io"
	"github.com/armon/go-socks5"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/FTwOoO/netstack/tcpip/stack"
	"github.com/FTwOoO/netstack/tcpip/buffer"
	"github.com/FTwOoO/netstack/tcpip/header"
	"github.com/FTwOoO/netstack/tcpip"
)

const socksAddr = "52.69.162.110:1080"

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

	linkId, err := tun2io.CreateFdLinkEndpoint(fd, mtu)
	if err != nil {
		log.Fatal(err)
	}

	s, err := tun2io.CreateStack(parsedAddr, linkId)
	if err != nil {
		log.Fatal(err)
	}

	dialer, err := tun2io.NewSOCKS5Dialer("tcp", socksAddr, nil)
	if err != nil {
		log.Fatal(err)
	}

	//go createSocks5Server(socksAddr)
	go generateUDPTest(s, linkId, 1)

	manager, err := tun2io.NewTun2ioManager(s, dialer)
	manager.MainLoop()
}

func createSocks5Server(addr string) {

	conf := &socks5.Config{}
	server, err := socks5.New(conf)
	if err != nil {
		panic(err)
	}

	if err := server.ListenAndServe("tcp", addr); err != nil {
		panic(err)
	}

}

func generateUDPTest(s tcpip.Stack, linkId tcpip.LinkEndpointID, NID tcpip.NICID) error {
	for {
		ep := stack.FindLinkEndpoint(linkId)
		if ep == nil {
			log.Fatalf("Endpoint not found:%d", linkId)
			return tcpip.ErrBadLinkEndpoint
		}

		d := s.(*stack.Stack).GetNic(NID)

		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{}
		gopacket.SerializeLayers(buf, opts,
			&layers.IPv4{SrcIP:net.IP{192, 168, 4, 1}, DstIP:net.IP{8, 8, 8, 8}, Protocol:layers.IPProtocolUDP},
			&layers.UDP{SrcPort:10089, DstPort:53},
			&layers.DNS{Questions:[]layers.DNSQuestion{
				layers.DNSQuestion{
					Name:[]byte("xhalee.info"),
					Type:layers.DNSTypeA,
					Class:layers.DNSClassIN}},
			},
		)


		packetData := buf.Bytes()

		log.Printf("genereated DNS req: %v\n", packetData)

		//hdr := buffer.NewPrependable(1024)
		//ep.WritePacket(nil, &hdr, packetData, header.IPv4ProtocolNumber)
		generateIpRequest(ep, d, header.IPv4ProtocolNumber, packetData)
		time.Sleep(10*time.Second)
	}

	return nil
}

func generateIpRequest(e stack.LinkEndpoint, d stack.NetworkDispatcher, p tcpip.NetworkProtocolNumber, packetData []byte) {

	view := buffer.View(packetData)
	vv := buffer.NewVectorisedView(len(packetData), []buffer.View{view})
	d.DeliverNetworkPacket(e, p, &vv)
}

