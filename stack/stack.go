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
	"net"
	"github.com/google/gopacket"
	"fmt"
)

// Stats holds statistics about the networking stack.
type Stats struct {
	// UnkownProtocolRcvdPackets is the number of packets received by the
	// stack that were for an unknown or unsupported protocol.
	UnknownProtocolRcvdPackets uint64

	// UnknownNetworkEndpointRcvdPackets is the number of packets received
	// by the stack that were for a supported network protocol, but whose
	// destination address didn't having a matching endpoint.
	UnknownNetworkEndpointRcvdPackets uint64

	// MalformedRcvPackets is the number of packets received by the stack
	// that were deemed malformed.
	MalformedRcvdPackets uint64
}

// FullAddress represents a full transport node address, as required by the
// Connect() and Bind() methods.
type FullAddress struct {
	// Addr is the network address.
	Addr NetworkAddress

	// Port is the transport port.
	Port TransportAddress
}

type Stack interface {

	// Stats returns a snapshot of the current stats.
	// TODO: Make stats available in sentry for debugging/diag.
	Stats() Stats

	// NICSubnets returns a map of NICIDs to their associated subnets.
	Subnets() []*net.IPNet

}

// Endpoint is the interface implemented by transport protocols (e.g., tcp, udp)
// that exposes functionality like read, write, connect, etc. to users of the
// networking stack.
type Endpoint interface {

	NewEndpoint()
	// Close puts the endpoint in a closed state and frees all resources
	// associated with it.
	Close()

	// Read reads data from the endpoint and optionally returns the sender.
	// This method does not block if there is no data pending.
	// It will also either return an error or data, never both.
	Read(*FullAddress) (View, error)

	// Write writes data to the endpoint's peer, or the provided address if
	// one is specified. This method does not block if the data cannot be
	// written.
	Write(View, *FullAddress) (uintptr, error)

	// RecvMsg reads data and a control message from the endpoint. This method
	// does not block if there is no data pending.
	RecvMsg(*FullAddress) (View, error)

	// SendMsg writes data and a control message to the endpoint's peer.
	// This method does not block if the data cannot be written.
	//
	// SendMsg does not take ownership of any of its arguments on error.
	SendMsg(View, *FullAddress) (uintptr, error)


	// Connect connects the endpoint to its peer. Specifying a NIC is
	// optional.
	//
	// There are three classes of return values:
	//	nil -- the attempt to connect succeeded.
	//	ErrConnectStarted -- the connect attempt started but hasn't
	//		completed yet. In this case, the actual result will
	//		become available via GetSockOpt(ErrorOption) when
	//		the endpoint becomes writable. (This mimics the
	//		connect(2) syscall behavior.)
	//	Anything else -- the attempt to connect failed.
	Connect(address FullAddress) error

	// ConnectEndpoint connects this endpoint directly to another.
	//
	// This should be called on the client endpoint, and the (bound)
	// endpoint passed in as a parameter.
	//
	// The error codes are the same as Connect.
	ConnectEndpoint(server Endpoint) error

	// Listen puts the endpoint in "listen" mode, which allows it to accept
	// new connections.
	Listen(backlog int) error

	// Accept returns a new endpoint if a peer has established a connection
	// to an endpoint previously set to listen mode. This method does not
	// block if no new connections are available.
	//
	// The returned Queue is the wait queue for the newly created endpoint.
	Accept() (Endpoint, error)

	// Bind binds the endpoint to a specific local address and port.
	// Specifying a NIC is optional.
	//
	// An optional commit function will be executed atomically with respect
	// to binding the endpoint. If this returns an error, the bind will not
	// occur and the error will be propagated back to the caller.
	Bind(address FullAddress, commit func() error) error

	// GetLocalAddress returns the address to which the endpoint is bound.
	GetLocalAddress() (FullAddress, error)

	// GetRemoteAddress returns the address to which the endpoint is
	// connected.
	GetRemoteAddress() (FullAddress, error)
}


type TransportEndpointID struct {
	LocalPort TransportAddress
	LocalAddress NetworkAddress
	RemotePort TransportAddress
	RemoteAddress NetworkAddress
}

func (id TransportEndpointID) ToString() string {
	return fmt.Sprintf("%v:%v -> %v:%v", id.RemoteAddress, id.RemotePort, id.LocalAddress, id.LocalPort)
}

type TransportEndpoint interface {
	// HandlePacket is called by the stack when new packets arrive to
	// this transport endpoint.
	HandlePacket(id TransportEndpointID, v View)
}

type TransportCallback func(netFlow gopacket.Flow, v View) bool


type Stack struct {
	Ip net.IP
	Subnet []*net.IPNet

	TcpCallback TransportCallback
	UdpCallback TransportCallback

	endpoints map[TransportEndpointID]TransportEndpoint
}


func (s *Stack) DeliverNetworkPacket() {

}