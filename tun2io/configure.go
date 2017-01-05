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
	"errors"
	"time"
	"github.com/FTwOoO/netstack/tcpip"
	"fmt"
)

var (
	errBufferIsFull = errors.New("Buffer is full.")
	errDeviceClosed = errors.New("Device is closed.")
	reasonClientAbort = "Aborted by client."
	ioTimeout = time.Second * 60
)

type TunnelStatus uint

const (
	StatusNew TunnelStatus = iota // 0
	StatusConnecting                     // 1
	StatusConnectionFailed               // 2
	StatusConnected                      // 3
	StatusProxying                       // 5
	StatusClosing                        // 6
	StatusClosed                         // 7

	readBufSize = 1024 * 64
)



type TransportID struct {
	Transport     tcpip.TransportProtocolNumber

	// srcPort is the src port from client
	SrcPort       uint16

	// srcAddress is the src [network layer] address associated with client.
	SrcAddress    tcpip.Address

	// RemotePort is the remote port associated with the target.
	RemotePort    uint16

	// RemoteAddress it the remote [network layer] address associated with
	// the target.
	RemoteAddress tcpip.Address
}

func (id TransportID) ToString() string {
	return fmt.Sprintf("%s:%d -> %s:%d", id.RemoteAddress, id.RemotePort, id.SrcAddress, id.SrcPort)
}
