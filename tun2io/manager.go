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
	"sync"
	"log"
	"time"
	"github.com/FTwOoO/netstack/tcpip"
	"github.com/FTwOoO/netstack/waiter"
	"github.com/FTwOoO/netstack/tcpip/stack"
	"github.com/FTwOoO/netstack/tcpip/header"
	"github.com/FTwOoO/netstack/tcpip/buffer"
	"golang.org/x/net/proxy"
	"github.com/athom/goset"
)

type Tun2ioManager struct {
	stack                  tcpip.Stack
	defaultDialer          proxy.Dialer

	tunnelsMu              sync.Mutex
	tunnels                map[TransportID]*Tunnel
	tcpListeners           map[TransportID]*TcpListener

	tcpListener2TcpTunnels map[TransportID][]TransportID

	NID                    tcpip.NICID
	Subnets                []tcpip.Subnet
}

func NewTun2ioManager(s tcpip.Stack, nicid tcpip.NICID, defaultDialer proxy.Dialer) (*Tun2ioManager, error) {

	m := &Tun2ioManager{
		stack:s,
		tunnels: make(map[TransportID]*Tunnel, 0),
		tcpListeners: make(map[TransportID]*TcpListener, 0),
		tcpListener2TcpTunnels: make(map[TransportID][]TransportID, 0),
		defaultDialer:defaultDialer,
		NID: nicid,
	}

	m.Subnets = s.NICSubnets()[nicid]

	s.(*stack.Stack).SetTransportProtocolHandler(header.TCPProtocolNumber, m.tcpHandler)
	s.(*stack.Stack).SetTransportProtocolHandler(header.UDPProtocolNumber, m.udpHandler)
	s.(*stack.Stack).SetForwardMode(true)
	return m, nil
}

func (m *Tun2ioManager) MainLoop() {
	for {
		time.Sleep(2 * time.Second)
		s := "\n==========================>\n"
		s += m.stack.(*stack.Stack).PrintNicTransportStats()
		s += m.GetDebugStats()
		s += "<--------------------------\n"
		log.Print(s)

	}
}

func (m *Tun2ioManager) GetDebugStats() string {
	var ret string = "tunnels:\n"

	for tid, _ := range m.tunnels {
		ret += tid.ToString() + "\n"
	}

	ret += "tcpListeners:\n"
	for tid, _ := range m.tcpListeners {
		ret += tid.ToString() + "\n"
	}

	ret += "tcpListener2TcpTunnels:\n"
	for tid, arr := range m.tcpListener2TcpTunnels {
		if len(arr) == 0 {
			continue
		}

		ret += tid.ToString() + "=>"
		for _, tid2 := range arr {
			ret += tid2.ToString() + "\n"
		}
	}

	return ret
}

func (m *Tun2ioManager) tcpHandler(r *stack.Route, id stack.TransportEndpointID, vv *buffer.VectorisedView) bool {
	protocol := header.TCPProtocolNumber
	netProto := r.NetProto

	listenId := id
	listenId.RemoteAddress = ""
	listenId.RemotePort = 0

	//ignore packets to local
	if m.isTargetLocal(id.LocalAddress) {
		log.Printf("Ignore packet of id %v\n", id.ToString())
		return false
	}

	demux := m.stack.(*stack.Stack).GetDemuxer(m.NID)
	if demux.IsEndpointExist(netProto, protocol, id) || demux.IsEndpointExist(netProto, protocol, listenId) {
		return false
	}

	listenerId := TransportID{Transport:protocol, RemoteAddress: id.LocalAddress, RemotePort:id.LocalPort}
	log.Printf("Create endpoint with id %s\n", listenerId.ToString())
	l, err := NewTcpListener(m.stack, m.NID, netProto, listenerId)
	if err != nil {
		log.Print(err)
		return false
	}
	m.tunnelsMu.Lock()
	m.tcpListeners[listenerId] = l
	m.tcpListener2TcpTunnels[listenerId] = make([]TransportID, 0)
	m.tunnelsMu.Unlock()

	go func() {
		for {
			n, wq, err := l.Accept()
			if err != nil && err == ErrTimeout {
				//
				// Check if all related endpoints accepted by listener are all closed,
				// if so, close the listener as well
				//
				m.tunnelsMu.Lock()
				if arr, ok := m.tcpListener2TcpTunnels[listenerId]; !ok || len(arr) == 0 {

					log.Print("Accept() timeout, and Closed!\n")
					l.Close()
					delete(m.tcpListeners, listenerId)
					m.tunnelsMu.Unlock()
					return
				}
				m.tunnelsMu.Unlock()

				continue

			} else if err != nil {
				l.Close()
				log.Fatalf("Accept() failed: %s", err)
				return
			} else {
				go m.tcpCb(listenerId, wq, n)
				continue
			}
		}
	}()

	nic := m.stack.(*stack.Stack).GetNic(m.NID)
	nic.DeliverTransportPacket(r, protocol, vv)
	return true
}

func (m *Tun2ioManager) tcpCb(listenerId TransportID, wq *waiter.Queue, ep tcpip.Endpoint) {
	tunnel, err := NewTunnel("tcp", wq, ep, m.defaultDialer, m.endpointClosed)
	if err != nil {
		log.Print(err)
		ep.Close()
		return
	}

	m.tunnelsMu.Lock()
	defer m.tunnelsMu.Unlock()

	m.tunnels[tunnel.Id] = tunnel
	arr := m.tcpListener2TcpTunnels[listenerId]
	m.tcpListener2TcpTunnels[listenerId] = append(arr, tunnel.Id)

	tunnel.Run()
}

func (m *Tun2ioManager) endpointClosed(id TransportID) {
	m.tunnelsMu.Lock()
	defer m.tunnelsMu.Unlock()

	delete(m.tunnels, id)

	if id.Transport == header.TCPProtocolNumber {
		listenerId := id
		listenerId.SrcPort = 0
		listenerId.SrcAddress = ""

		if arr, ok := m.tcpListener2TcpTunnels[listenerId]; ok {
			if goset.IsIncluded(arr, id) {
				m.tcpListener2TcpTunnels[listenerId] = goset.RemoveElement(arr, id).([]TransportID)
			}
		}
	}
}

func (m *Tun2ioManager) udpHandler(r *stack.Route, id stack.TransportEndpointID, vv *buffer.VectorisedView) bool {
	protocol := header.UDPProtocolNumber
	netProto := r.NetProto

	if m.isTargetLocal(id.LocalAddress) {
		log.Printf("Ignore packet of id %v\n", id.ToString())
		return false
	}

	demux := m.stack.(*stack.Stack).GetDemuxer(m.NID)
	if demux.IsEndpointExist(netProto, protocol, id) {
		return false
	}

	log.Printf("Create endpoint with id %s\n", id.ToString())

	var wq waiter.Queue
	ep, err := m.stack.NewEndpoint(protocol, netProto, &wq)
	if err != nil {
		log.Fatal(err)
		return false
	}

	if err := ep.Bind(tcpip.FullAddress{m.NID, id.LocalAddress, id.LocalPort}, nil); err != nil {
		log.Fatal("Bind failed2: ", err)
		return false
	}

	if err := ep.Connect(tcpip.FullAddress{m.NID, id.RemoteAddress, id.RemotePort}); err != nil {
		log.Fatal("Connect failed: ", err)
		return false
	}

	tunnel, err := NewTunnel("udp", &wq, ep, m.defaultDialer, m.endpointClosed)
	if err != nil {
		log.Print(err)
		ep.Close()
		return false
	}

	m.tunnelsMu.Lock()
	m.tunnels[tunnel.Id] = tunnel
	m.tunnelsMu.Unlock()

	tunnel.Run()
	nic := m.stack.(*stack.Stack).GetNic(m.NID)
	nic.DeliverTransportPacket(r, protocol, vv)
	return true
}

func (m *Tun2ioManager) isTargetLocal(addr tcpip.Address) bool {
	for _, sn := range m.Subnets {
		if sn.Contains(addr) {
			return true
		}
	}
	return false
}