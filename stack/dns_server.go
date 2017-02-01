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

package stack

import (
	"github.com/miekg/dns"
	"context"
	"sync"
	"log"
	"net"
	"github.com/FTwOoO/netstack/tcpip"
)

type sessionWriter struct {
	remoteAddr tcpip.FullAddress
	writeChan  chan <- UdpPacket
}

func NewSessionWriter(remoteAddr tcpip.FullAddress, writeChan chan <- UdpPacket) (*sessionWriter, error) {
	return &sessionWriter{remoteAddr:remoteAddr, writeChan:writeChan}, nil
}

// WriteMsg implements the ResponseWriter.WriteMsg method.
func (w *sessionWriter) WriteMsg(m *dns.Msg) (err error) {
	var data []byte
	data, err = m.Pack()
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}

// Write implements the ResponseWriter.Write method.
func (w *sessionWriter) Write(m []byte) (int, error) {
	w.writeChan <- UdpPacket{Data:m, Addr:w.remoteAddr}
	return len(m), nil

}

// LocalAddr implements the ResponseWriter.LocalAddr method.
func (w *sessionWriter) LocalAddr() net.Addr {
	return &net.UDPAddr{IP:[]byte{}, Port:0, Zone:""}
}

// RemoteAddr implements the ResponseWriter.RemoteAddr method.
func (w *sessionWriter) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP:net.ParseIP(w.remoteAddr.Addr.String()), Port:int(w.remoteAddr.Port)}
}

// TsigStatus implements the ResponseWriter.TsigStatus method.
func (w *sessionWriter) TsigStatus() error { return nil }

// TsigTimersOnly implements the ResponseWriter.TsigTimersOnly method.
func (w *sessionWriter) TsigTimersOnly(b bool) {}

// Hijack implements the ResponseWriter.Hijack method.
func (w *sessionWriter) Hijack() {}

// Close implements the ResponseWriter.Close method
func (w *sessionWriter) Close() error {
	return nil
}


type DnsServer struct {
	udpEp     *UdpEndpoint
	Handler   dns.Handler

	ctx       context.Context
	ctxCancel context.CancelFunc
	closeOne  sync.Once
}

func CreateDnsServer(udpEp *UdpEndpoint, handler dns.Handler) (*DnsServer, error) {
	d := &DnsServer{udpEp:udpEp, Handler:handler}
	d.ctx, d.ctxCancel = context.WithCancel(context.Background())

	go d.reader()
	return d, nil
}

func (d *DnsServer) reader() {
	Reading:for {
		select {
		case udpPacket := <-d.udpEp.RecvPackets:
			w, _ := NewSessionWriter(udpPacket.Addr, d.udpEp.WritePackets)

			req := new(dns.Msg)
			err := req.Unpack(udpPacket.Data)
			if err != nil {
				x := new(dns.Msg)
				x.SetRcodeFormatError(req)
				w.WriteMsg(x)
				w.Close()
			}


			if d.Handler != nil {
				d.Handler.ServeDNS(w, req)
			}
			w.Close()


		case <-d.ctx.Done():
			log.Printf("reader done because of '%s'", d.ctx.Err())
			break Reading
		}
	}
}


func (d *DnsServer) Close(reason error) error {
	d.closeOne.Do(func() {
		log.Printf("DnsServer Closed:%s\n", reason.Error())
		d.udpEp.Close(reason)
		d.ctxCancel()

	})

	return nil
}