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
	"net"
	"golang.org/x/net/context"
	"sync"
	"time"
	"github.com/FTwOoO/netstack/tcpip"
	"github.com/FTwOoO/netstack/waiter"
	"golang.org/x/net/proxy"
	"log"
	"fmt"
	"github.com/FTwOoO/netstack/tcpip/header"
)

type Tunnel struct {
	Id               TransportID
	wq               *waiter.Queue
	ep               tcpip.Endpoint

	connOut          net.Conn

	status           TunnelStatus
	statusMu         sync.Mutex

	tunnelRecvChunks chan []byte
	recvChunks       chan []byte

	ctx              context.Context
	ctxCancel        context.CancelFunc
	closeCallback    func(TransportID)

	quitOne          sync.Once
}

func NewTunnel(network string, wq *waiter.Queue, ep tcpip.Endpoint, dialer proxy.Dialer, closeCallback func(TransportID)) (*Tunnel, error) {
	srcAddr, _ := ep.GetRemoteAddress()
	remoteAddr, _ := ep.GetLocalAddress()

	id := TransportID{0, srcAddr.Port, srcAddr.Addr, remoteAddr.Port, remoteAddr.Addr}

	if network == "tcp" {
		id.Transport = header.TCPProtocolNumber
	} else if network == "udp" {
		id.Transport = header.UDPProtocolNumber
	}

	t := &Tunnel{
		Id:id,
		wq:wq,
		ep:ep,
		tunnelRecvChunks:make(chan []byte, 256),
		recvChunks:make(chan []byte, 256),
		closeCallback: closeCallback,
	}

	t.SetStatus(StatusConnecting)

	var err error
	targetAddr := fmt.Sprintf("%s:%d", id.RemoteAddress, id.RemotePort)
	log.Printf("Try to connect to %s by proto %s\n", targetAddr, network)
	if t.connOut, err = dialer.Dial(network, targetAddr); err != nil {
		t.SetStatus(StatusConnectionFailed)
		return nil, err
	}

	t.SetStatus(StatusConnected)

	return t, nil
}

func (t *Tunnel) Run() {

	t.ctx, t.ctxCancel = context.WithCancel(context.Background())
	go t.reader()
	go t.writer()
	go t.tunnelReader()
	go t.tunnelWriter()
	t.SetStatus(StatusProxying)
}

func (t *Tunnel) SetStatus(s TunnelStatus) {
	t.statusMu.Lock()
	t.status = s
	t.statusMu.Unlock()
}

func (t *Tunnel) Status() TunnelStatus {
	t.statusMu.Lock()
	s := t.status
	t.statusMu.Unlock()
	return s
}

func (t *Tunnel) reader() {
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)

	t.wq.EventRegister(&waitEntry, waiter.EventIn)
	defer t.wq.EventUnregister(&waitEntry)

	for {
		v, err := t.ep.Read(nil)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifyCh
				continue
			}

			log.Print(err)
			t.quit(err.Error())
			return
		}

		t.recvChunks <- v
	}

	return
}

func (t *Tunnel) writer() {
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)

	t.wq.EventRegister(&waitEntry, waiter.EventIn)
	defer t.wq.EventUnregister(&waitEntry)

	for {
		select {
		case <-t.ctx.Done():
			log.Printf("writer done because %s", t.ctx.Err())
			return
		case chunk := <-t.tunnelRecvChunks:
			for {
				_, err := t.ep.Write(chunk, nil)
				if err != nil {
					if err == tcpip.ErrWouldBlock {
						<-notifyCh
						continue
					}

					log.Print(err)
					t.quit(err.Error())
					return

				} else {
					break
				}
			}
		}
	}

	return
}

func (t *Tunnel) tunnelReader() {

	for {
		select {
		case <-t.ctx.Done():
			log.Printf("tunnel reader done because %s", t.ctx.Err())
			return

		default:
			data := make([]byte, readBufSize)
			t.connOut.SetReadDeadline(time.Now().Add(ioTimeout))
			n, err := t.connOut.Read(data)
			if err != nil {
				log.Print(err)
				t.quit(err.Error())
				return
			}
			if n > 0 {
				t.tunnelRecvChunks <- data[0:n]
			}
		}
	}

	return
}

func (t *Tunnel) tunnelWriter() {

	for {
		select {
		case <-t.ctx.Done():
			log.Printf("tunnel writer done because %s", t.ctx.Err())
			return

		case chunk := <-t.recvChunks:
			for {
				n, err := t.connOut.Write(chunk)
				if err != nil {
					log.Print(err)
					t.quit(err.Error())
					return
				}

				if n < len(chunk) {
					chunk = chunk[n:]
					continue
				}

				break
			}

		}
	}

	return
}

func (t *Tunnel) quit(reason string)  {

	t.quitOne.Do(func() {
		status := t.Status()

		if status != StatusProxying {
			log.Printf("unexpected status %d", status)
		}

		t.SetStatus(StatusClosing)
		t.ctxCancel()
		t.connOut.Close()
		t.ep.Close()

		t.SetStatus(StatusClosed)
		if t.closeCallback != nil {
			t.closeCallback(t.Id)
		}
	})

	return
}