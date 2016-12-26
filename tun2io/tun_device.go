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
	"C"
	"net"
	"sync"
	"sync/atomic"
)

type TunDevice struct {
	Tunnels       map[uint32]*TcpTunnel
	TunnelMu      sync.Mutex
	defaultDialer Dialer
	readChan      chan []byte
	closed        int
}

func NewDevice(addr net.IP, mask net.IPMask, dialer Dialer) (d *TunDevice, err error) {
	d = &TunDevice{
		Tunnels : make(map[uint32]*TcpTunnel),
		defaultDialer:dialer,
		readChan: make(chan []byte, 1024),
	}
	cInitLwip(addr, mask)
	goSetDevice(d)

	return d, nil
}

func (d *TunDevice)Dialer() Dialer {
	return d.defaultDialer
}

func (d *TunDevice)Close() {
	atomic.StoreInt32(&d.closed, 1)
	cLwipDestrop()
	close(d.readChan)
}

func (d *TunDevice)Write(b []byte) {
	//TODO
	cLwipWrite(b)
}

func (d *TunDevice)Read() (b []byte, err error) {
	if d.closed {
		return nil, errDeviceClosed
	}

	b = <-d.readChan
	if b == nil {
		return nil, errDeviceClosed
	}

	return b, nil

}