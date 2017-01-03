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
	"github.com/FTwOoO/netstack/tcpip"
	"github.com/FTwOoO/netstack/waiter"
	"sync"
	"log"
	"time"
	"golang.org/x/net/proxy"
)

type Tun2ioManager struct {
	stack         tcpip.Stack
	defaultDialer proxy.Dialer

	tcpTunnelsMu  sync.Mutex
	tcpTunnels    map[TransportID]*TcpTunnel

	udpTunnelsMu  sync.Mutex
	udpTunnels    map[TransportID]*UdpTunnel
}

func NewTun2ioManager(s tcpip.Stack, defaultDialer proxy.Dialer) (*Tun2ioManager, error) {

	m := &Tun2ioManager{
		stack:s,
		tcpTunnels: make(map[TransportID]*TcpTunnel, 0),
		udpTunnels: make(map[TransportID]*UdpTunnel, 0),
		defaultDialer:defaultDialer,
	}
	s.SetForwardMode(true, m.tcpCallback, m.udpCallback)

	return m, nil
}

func (m *Tun2ioManager) MainLoop() {
	for {
		time.Sleep(5 * time.Second)
		log.Printf(m.stack.PrintNicTransportStats())
	}
}

func (m *Tun2ioManager) tcpCallback(wq *waiter.Queue, ep tcpip.Endpoint) {

	tunnel, err := NewTcpTunnel(wq, ep, m.defaultDialer, m.tcpEndpointClosed)
	if err != nil {
		log.Print(err)
		ep.Close()
		return
	}

	m.tcpTunnelsMu.Lock()
	defer m.tcpTunnelsMu.Unlock()
	m.tcpTunnels[tunnel.Id] = tunnel

	tunnel.Run()
}

func (m *Tun2ioManager) tcpEndpointClosed(id TransportID) {
	m.tcpTunnelsMu.Lock()
	defer m.tcpTunnelsMu.Unlock()
	delete(m.tcpTunnels, id)
}

func (m *Tun2ioManager) udpCallback(wq *waiter.Queue, ep tcpip.Endpoint) {
}

