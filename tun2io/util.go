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

package tun2io

import (
	"strings"
	"log"
	"fmt"
	"net"
	"github.com/FTwOoO/netstack/tcpip"
	"github.com/FTwOoO/netstack/tcpip/link/fdbased"
	"github.com/FTwOoO/netstack/tcpip/network/ipv4"
	"github.com/FTwOoO/netstack/tcpip/network/ipv6"
	"github.com/FTwOoO/netstack/tcpip/stack"
	"github.com/FTwOoO/netstack/tcpip/transport/tcp"
	"github.com/FTwOoO/netstack/waiter"
	"github.com/FTwOoO/netstack/tcpip/transport/udp"
)

func CreateFdLinkEndpoint(fd int, mtu int) (tcpip.LinkEndpointID, error) {
	linkID := fdbased.New(fd, mtu, nil)
	return linkID, nil
}


func CreateStack(mainAddr net.IP, linkEndpointId tcpip.LinkEndpointID) (tcpip.Stack, error) {
	var addr tcpip.Address
	var proto tcpip.NetworkProtocolNumber

	if mainAddr.To4() != nil {
		addr = tcpip.Address(mainAddr.To4())
		proto = ipv4.ProtocolNumber
	} else if mainAddr.To16() != nil {
		addr = tcpip.Address(mainAddr.To16())
		proto = ipv6.ProtocolNumber
	} else {
		err := fmt.Errorf("Unknown IP type: %v", mainAddr)
		log.Fatal(err)
		return nil, err
	}


	// Create the stack with ip and tcp protocols, then add a tun-based
	// NIC and address.
	s := stack.New([]string{ipv4.ProtocolName, ipv6.ProtocolName}, []string{tcp.ProtocolName, udp.ProtocolName})
	if err := s.CreateNIC(1, linkEndpointId); err != nil {
		log.Fatal(err)
		return nil, err
	}


	if err := s.AddAddress(1, proto, addr); err != nil {
		log.Fatal(err)
		return nil, err
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

	return s, nil
}

func TcpEcho(wq *waiter.Queue, ep tcpip.Endpoint) {
	defer ep.Close()

	// Create wait queue entry that notifies a channel.
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)

	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		v, err := ep.Read(nil)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifyCh
				continue
			}

			return
		}

		ep.Write(v, nil)
	}
}
