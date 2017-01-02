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

	linkId, err := tun2io.CreateFdLinkEndpoint(fd, mtu)
	if err != nil {
		log.Fatal(err)
	}

	s, err := tun2io.CreateStack(parsedAddr, linkId)
	if err != nil {
		log.Fatal(err)
	}

	manager, err := tun2io.NewTun2ioManager(s, tun2io.TcpDirectDialer)
	manager.MainLoop()
}



