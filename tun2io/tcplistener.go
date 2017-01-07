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
	"github.com/FTwOoO/netstack/tcpip"
	"github.com/FTwOoO/netstack/waiter"
	"github.com/FTwOoO/netstack/tcpip/header"
	"golang.org/x/net/context"
	"time"
)

type TcpListener struct {
	endpoint      tcpip.Endpoint
	remoteAddress tcpip.Address
	remotePort    uint16

	notifyCh      chan struct{}
	wq            *waiter.Queue
	waitEndry     waiter.Entry

	ctx           context.Context
	ctxCancel     context.CancelFunc
	quitOne       sync.Once
}

func NewTcpListener(s tcpip.Stack, nid tcpip.NICID, netProto tcpip.NetworkProtocolNumber, listenerId TransportID) (m *TcpListener, err error) {

	protocol := header.TCPProtocolNumber
	var wq waiter.Queue
	ep, err := s.NewEndpoint(protocol, netProto, &wq)
	if err != nil {
		return nil, err
	}

	if err = ep.Bind(tcpip.FullAddress{nid, listenerId.RemoteAddress, listenerId.RemotePort}, nil); err != nil {
		return nil, err
	}

	if err = ep.Listen(10); err != nil {
		return nil, err
	}

	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)

	m = &TcpListener{
		endpoint:ep,
		remoteAddress:listenerId.RemoteAddress,
		remotePort:listenerId.RemotePort,
		notifyCh:notifyCh,
		wq:&wq,
		waitEndry:waitEntry,
	}
	m.ctx, m.ctxCancel = context.WithCancel(context.Background())

	return
}

// The queue returned is own by returned endpoint(private element)
func (t *TcpListener) Accept() (tcpip.Endpoint, *waiter.Queue, error) {

	AcceptLoop:for {
		n, wq, err := t.endpoint.Accept()
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				select {
				case <-t.notifyCh:
					continue AcceptLoop
				case <-t.ctx.Done():
					return nil, nil, t.ctx.Err()
				case <- time.After(listenTimeout):
					return nil, nil, ErrTimeout
				}
			} else {
				return nil, nil, err
			}
		}

		l, _ := n.GetLocalAddress()
		r, _ := n.GetRemoteAddress()
		log.Printf("Accept a connection from %s:%d->%s:%d\n", r.Addr, r.Port, l.Addr, l.Port)
		return n, wq, nil
	}

}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (t *TcpListener) Close() error {
	t.quitOne.Do(func() {
		t.wq.EventUnregister(&t.waitEndry)
		t.endpoint.Close()
		t.ctxCancel()
	})

	return nil
}
