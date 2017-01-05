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
)

type Tun2ioManager struct {
	stack         tcpip.Stack
	defaultDialer proxy.Dialer

	tunnelsMu     sync.Mutex
	tunnels       map[TransportID]*Tunnel

	NID           tcpip.NICID
}

func NewTun2ioManager(s tcpip.Stack, defaultDialer proxy.Dialer) (*Tun2ioManager, error) {

	m := &Tun2ioManager{
		stack:s,
		tunnels: make(map[TransportID]*Tunnel, 0),
		defaultDialer:defaultDialer,
		NID: 1,
	}

	s.(*stack.Stack).SetTransportProtocolHandler(header.TCPProtocolNumber, m.tcpHandler)
	s.(*stack.Stack).SetTransportProtocolHandler(header.UDPProtocolNumber, m.udpHandler)
	s.(*stack.Stack).SetForwardMode(true)
	return m, nil
}

func (m *Tun2ioManager) MainLoop() {
	for {
		time.Sleep(5 * time.Second)
		log.Printf(m.stack.(*stack.Stack).PrintNicTransportStats())
	}
}

func (m *Tun2ioManager) tcpHandler(r *stack.Route, id stack.TransportEndpointID, vv *buffer.VectorisedView) bool {
	protocol := header.TCPProtocolNumber
	netProto := r.NetProto

	//TODO: check if its local ip/local subnet ip
	listenId := id
	listenId.RemoteAddress = ""
	listenId.RemotePort = 0

	log.Printf("Try to find endpoint for id[%s] and listen id[%s]\n", id.ToString(), listenId.ToString())

	demux := m.stack.(*stack.Stack).GetDemuxer(m.NID)
	if demux.IsEndpointExist(netProto, protocol, id) || demux.IsEndpointExist(netProto, protocol, listenId) {
		return false
	}

	log.Printf("Create endpoint with id %s\n", id.ToString())

	var wq waiter.Queue
	ep, err := m.stack.NewEndpoint(protocol, netProto, &wq)
	if err != nil {
		log.Fatal(err)
		return false
	}

	if err := ep.Bind(tcpip.FullAddress{m.NID, listenId.LocalAddress, listenId.LocalPort}, nil); err != nil {
		log.Fatal("Bind failed: ", err)
		return false
	}

	if err := ep.Listen(10); err != nil {
		log.Fatal("Listen failed: ", err)
		return false

	}

	go func() {
		waitEntry, notifyCh := waiter.NewChannelEntry(nil)
		wq.EventRegister(&waitEntry, waiter.EventIn)
		defer wq.EventUnregister(&waitEntry)

		for {
			n, wq, err := ep.Accept()
			if err != nil {
				if err == tcpip.ErrWouldBlock {
					<-notifyCh
					continue
				}

				log.Fatalf("Accept() failed: current %s", err)
			}

			l, _ := n.GetLocalAddress()
			r, _ := n.GetRemoteAddress()

			log.Printf("Accept a connection from %s:%d->%s:%d\n", r.Addr, r.Port, l.Addr, l.Port)
			go m.tcpCb(wq, n)
		}
	}()

	nic := m.stack.(*stack.Stack).GetNic(m.NID)
	nic.DeliverTransportPacket(r, protocol, vv)
	return true
}

func (m *Tun2ioManager) tcpCb(wq *waiter.Queue, ep tcpip.Endpoint) {
	tunnel, err := NewTunnel("tcp", wq, ep, m.defaultDialer, m.endpointClosed)
	if err != nil {
		log.Print(err)
		ep.Close()
		return
	}

	m.tunnelsMu.Lock()
	defer m.tunnelsMu.Unlock()
	m.tunnels[tunnel.Id] = tunnel

	tunnel.Run()
}

func (m *Tun2ioManager) endpointClosed(id TransportID) {
	m.tunnelsMu.Lock()
	defer m.tunnelsMu.Unlock()
	delete(m.tunnels, id)
}

func (m *Tun2ioManager) udpHandler(r *stack.Route, id stack.TransportEndpointID, vv *buffer.VectorisedView) bool {
	log.Printf("Receive a unhandle UDP packet!\n")

	protocol := header.UDPProtocolNumber
	netProto := r.NetProto


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

	//id.LocalAddress:id.LocalPort is the target ip-port pair
	if err := ep.Bind(tcpip.FullAddress{m.NID, id.LocalAddress, id.LocalPort}, nil); err != nil {
		log.Fatal("Bind failed: ", err)
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
	defer m.tunnelsMu.Unlock()
	m.tunnels[tunnel.Id] = tunnel

	tunnel.Run()
	nic := m.stack.(*stack.Stack).GetNic(m.NID)
	nic.DeliverTransportPacket(r, protocol, vv)
	return true
}
