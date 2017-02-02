package stack

import (
	"math"
	"math/rand"
	"sync"
	"errors"
	"github.com/google/gopacket"
)


// NetworkProtocolNumber is the number of a network protocol.
type NetworkProtocolNumber uint32

// Errors that can be returned by the network stack.
var (
	ErrUnknownProtocol = errors.New("unknown protocol")
	ErrUnknownNICID = errors.New("unknown nic id")
	ErrDuplicateNICID = errors.New("duplicate nic id")
	ErrDuplicateAddress = errors.New("duplicate address")
	ErrNoRoute = errors.New("no route")
	ErrBadLinkEndpoint = errors.New("bad link layer endpoint")
	ErrAlreadyBound = errors.New("endpoint already bound")
	ErrInvalidEndpointState = errors.New("endpoint is in invalid state")
	ErrAlreadyConnecting = errors.New("endpoint is already connecting")
	ErrAlreadyConnected = errors.New("endpoint is already connected")
	ErrNoPortAvailable = errors.New("no ports are available")
	ErrPortInUse = errors.New("port is in use")
	ErrBadLocalAddress = errors.New("bad local address")
	ErrClosedForSend = errors.New("endpoint is closed for send")
	ErrClosedForReceive = errors.New("endpoint is closed for receive")
	ErrWouldBlock = errors.New("operation would block")
	ErrConnectionRefused = errors.New("connection was refused")
	ErrTimeout = errors.New("operation timed out")
	ErrAborted = errors.New("operation aborted")
	ErrConnectStarted = errors.New("connection attempt started")
	ErrDestinationRequired = errors.New("destination address is required")
	ErrNotSupported = errors.New("operation not supported")
	ErrNotConnected = errors.New("endpoint not connected")
	ErrConnectionReset = errors.New("connection reset by peer")
	ErrConnectionAborted = errors.New("connection aborted")
)

const (
	// firstEphemeral is the first ephemeral port.
	firstEphemeral uint16 = 16000
)

type NetworkAddress struct {
	Type gopacket.LayerType
	Len  int
	Raw  [gopacket.MaxEndpointSize]byte
}

type TransportAddress struct {
	Transport gopacket.LayerType
	Port      uint16
}

// PortManager manages allocating, reserving and releasing ports.
type PortManager struct {
	mu             sync.RWMutex
	allocatedPorts map[TransportAddress]bindAddresses
}

// bindAddresses is a set of IP addresses.
type bindAddresses map[NetworkAddress]struct{}

// isAvailable checks whether an IP address is available to bind to.
func (b bindAddresses) isAvailable(addr NetworkAddress) bool {
	if _, ok := b[addr]; ok {
		return false
	}
	return true
}

// NewPortManager creates new PortManager.
func NewPortManager() *PortManager {
	return &PortManager{allocatedPorts: make(map[TransportAddress]bindAddresses)}
}

// PickEphemeralPort randomly chooses a starting point and iterates over all
// possible ephemeral ports, allowing the caller to decide whether a given port
// is suitable for its needs, and stopping when a port is found or an error
// occurs.
func (s *PortManager) PickEphemeralPort(testPort func(p uint16) (bool, error)) (port uint16, err error) {
	count := uint16(math.MaxUint16 - firstEphemeral + 1)
	offset := uint16(rand.Int31n(int32(count)))

	for i := uint16(0); i < count; i++ {
		port = firstEphemeral + (offset + i) % count
		ok, err := testPort(port)
		if err != nil {
			return 0, err
		}

		if ok {
			return port, nil
		}
	}

	return 0, ErrNoPortAvailable
}

// ReservePort marks a port/IP combination as reserved so that it cannot be
// reserved by another endpoint. If port is zero, ReservePort will search for
// an unreserved ephemeral port and reserve it, returning its value in the
// "port" return value.
func (s *PortManager) ReservePort(addr NetworkAddress, transport TransportAddress) (reservedPort uint16, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If a port is specified, just try to reserve it for all network
	// protocols.
	if transport.Port != 0 {
		if !s.reserveSpecificPort(transport, addr) {
			return 0, ErrPortInUse
		}
		return transport.Port, nil
	}

	// A port wasn't specified, so try to find one.
	return s.PickEphemeralPort(func(p uint16) (bool, error) {
		return s.reserveSpecificPort(transport, addr), nil
	})
}

// reserveSpecificPort tries to reserve the given port on all given protocols.
func (s *PortManager) reserveSpecificPort(transport TransportAddress, addr NetworkAddress) bool {
	// Check that the port is available on all network protocols.
	if addrs, ok := s.allocatedPorts[transport]; ok {
		if !addrs.isAvailable(addr) {
			return false
		}
	}

	m, ok := s.allocatedPorts[transport]
	if !ok {
		m = make(bindAddresses)
		s.allocatedPorts[transport] = m
	}
	m[addr] = struct{}{}

	return true
}

// ReleasePort releases the reservation on a port/IP combination so that it can
// be reserved by other endpoints.
func (s *PortManager) ReleasePort(transport TransportAddress, addr NetworkAddress) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m := s.allocatedPorts[transport]
	delete(m, addr)
	if len(m) == 0 {
		delete(s.allocatedPorts, transport)
	}

}
