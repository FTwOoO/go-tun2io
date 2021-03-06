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
	"golang.org/x/net/proxy"
	"errors"
)

type DirectDialer struct{}

func (f *DirectDialer) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}

type SOCKS5Dialer struct {
	Auth      *proxy.Auth
	SocksAddr string
}

func (f *SOCKS5Dialer) Dial(network, addr string) (net.Conn, error) {
	if network == "udp" {
		return new(DirectDialer).Dial(network, addr)
	} else if network == "tcp" {

		dialer, err := proxy.SOCKS5(network, f.SocksAddr, f.Auth, new(DirectDialer))
		if err != nil {
			return nil, err
		}

		return dialer.Dial(network, addr)
	}

	return nil, errors.New("Unsupported network type")
}
