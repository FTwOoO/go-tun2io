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
	"context"
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
	Id                TransportID
	wq                *waiter.Queue
	ep                tcpip.Endpoint

	connOut           net.Conn

	status            TunnelStatus
	statusMu          sync.Mutex

	tunnelRecvPackets chan []byte
	recvPackets       chan []byte

	ctx               context.Context
	ctxCancel         context.CancelFunc
	closeCallback     func(TransportID)

	closeOne          sync.Once
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
		tunnelRecvPackets:make(chan []byte, 256),
		recvPackets:make(chan []byte, 256),
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

	Reading:for {
		v, err := t.ep.Read(nil)
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
			t.recvPackets <- v
			continue Reading
		}
	}

	return
}

func (t *Tunnel) writer() {
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)

	t.wq.EventRegister(&waitEntry, waiter.EventOut)
	defer t.wq.EventUnregister(&waitEntry)

	Writing:for {
		select {
		case <-t.ctx.Done():
			log.Printf("writer done because of '%s'", t.ctx.Err())
			break Writing
		case chunk := <-t.tunnelRecvPackets:
			Write1Packet:for {
				_, err := t.ep.Write(chunk, nil)
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

func (t *Tunnel) tunnelReader() {

	Reading:for {
		select {
		case <-t.ctx.Done():
			log.Printf("tunnel reader done because of '%s'", t.ctx.Err())
			break Reading

		default:
			data := make([]byte, readBufSize)
			t.connOut.SetReadDeadline(time.Now().Add(readTimeout))
			n, err := t.connOut.Read(data)
			if err != nil {
				t.Close(err)
				break Reading
			}
			if n > 0 {
				log.Printf("receive a packet from tunnel[%s]\n", t.Id.ToString())
				t.tunnelRecvPackets <- data[0:n]
			}
		}
	}

	return
}

func (t *Tunnel) tunnelWriter() {

	Writing:for {
		select {
		case <-t.ctx.Done():
			log.Printf("tunnel writer done because of '%s'", t.ctx.Err())
			break Writing
		case chunk := <-t.recvPackets:
			Write1Packet:for {
				t.connOut.SetWriteDeadline(time.Now().Add(writeTimeout))
				n, err := t.connOut.Write(chunk)
				if err != nil {
					t.Close(err)
					break Writing
				} else if n < len(chunk) {
					chunk = chunk[n:]
					continue Write1Packet
				} else {
					log.Printf("Write a packet to tunnel[%s]\n", t.Id.ToString())
					break Write1Packet
				}
			}
		}
	}

	return
}

func (t *Tunnel) Close(reason error) {

	t.closeOne.Do(func() {
		status := t.Status()

		if status != StatusProxying {
			log.Printf("unexpected status %d", status)
		}

		log.Printf("%s\n", reason.Error())
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