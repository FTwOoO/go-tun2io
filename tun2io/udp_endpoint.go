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
	"log"
	"github.com/FTwOoO/netstack/tcpip"
	"github.com/FTwOoO/netstack/waiter"
	"github.com/FTwOoO/netstack/tcpip/transport/udp"
	"sync"
	"context"
	"time"
)

type UdpPacket struct {
	Addr tcpip.FullAddress
	Data []byte
}

type UdpEndpoint struct {
	endpoint     tcpip.Endpoint
	bindAddr     tcpip.FullAddress

	notifyCh     chan struct{}
	wq           *waiter.Queue
	RecvPackets  chan UdpPacket
	WritePackets chan UdpPacket

	ctx          context.Context
	ctxCancel    context.CancelFunc
	quitOne      sync.Once
}

func CreateUdpEndpoint(s tcpip.Stack, netProto tcpip.NetworkProtocolNumber, addr tcpip.FullAddress) (*UdpEndpoint, error) {

	var wq waiter.Queue
	ep, err := s.NewEndpoint(udp.ProtocolNumber, netProto, &wq)
	if err != nil {
		log.Fatal(err)
	}

	defer ep.Close()

	if err := ep.Bind(addr, nil); err != nil {
		log.Fatal("Bind failed: ", err)
	}

	u := &UdpEndpoint{endpoint:ep,
		bindAddr:addr,
		wq:&wq,
		RecvPackets:make(chan UdpPacket, 100),
		WritePackets:make(chan UdpPacket, 100),
	}
	u.ctx, u.ctxCancel = context.WithCancel(context.Background())

	go u.reader()
	go u.writer()

	return u, nil
}

func (t *UdpEndpoint) reader() {
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)

	t.wq.EventRegister(&waitEntry, waiter.EventIn)
	defer t.wq.EventUnregister(&waitEntry)

	Reading:for {
		var fromAddr tcpip.FullAddress
		v, err := t.endpoint.Read(&fromAddr)
		if err != nil && err == tcpip.ErrWouldBlock {
			select {
			case <-t.ctx.Done():
				log.Printf("reader done because of '%s'", t.ctx.Err())
				break Reading
			case <-notifyCh:
				continue Reading
			case <-time.After(readTimeout):
				t.Close(ErrTimeout)
				break Reading
			}
		} else if err != nil {
			t.Close(err)
			break Reading
		} else {
			t.RecvPackets <- UdpPacket{Data:v, Addr:fromAddr}
			continue Reading
		}
	}

	return
}

func (t *UdpEndpoint) writer() {
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)

	t.wq.EventRegister(&waitEntry, waiter.EventOut)
	defer t.wq.EventUnregister(&waitEntry)

	Writing:for {
		select {
		case <-t.ctx.Done():
			log.Printf("writer done because of '%s'", t.ctx.Err())
			break Writing
		case udpPacket := <-t.WritePackets:
			Write1Packet:for {
				_, err := t.endpoint.Write(udpPacket.Data, &udpPacket.Addr)
				if err != nil && err == tcpip.ErrWouldBlock {
					select {
					case <-t.ctx.Done():
						log.Printf("writer done because of '%s'", t.ctx.Err())
						break Writing
					case <-notifyCh:
						continue Write1Packet
					case <-time.After(writeTimeout):
						t.Close(ErrTimeout)
						break Writing
					}
				} else if err != nil {
					t.Close(err)
					break Writing

				} else {
					break Write1Packet
				}
			}
		}
	}

	return
}

func (t *UdpEndpoint) Close(reason error) error {
	t.quitOne.Do(func() {
		log.Printf("UdpEndpoint Closed:%s\n", reason.Error())
		t.endpoint.Close()
		t.ctxCancel()
	})

	return nil
}