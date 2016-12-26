package tun2io

import (
	"errors"
	"net"
	"time"
)

var (
	errBufferIsFull = errors.New("Buffer is full.")
	errDeviceClosed = errors.New("Device is closed.")
	reasonClientAbort = "Aborted by client."
	ioTimeout = time.Second * 30
)

const (
	StatusNew Status = iota // 0
	StatusConnecting                     // 1
	StatusConnectionFailed               // 2
	StatusConnected                      // 3
	StatusProxying                       // 5
	StatusClosing                        // 6
	StatusClosed                         // 7

	readBufSize = 1024 * 64
)

type Addr struct {
	addr net.Addr
	port int
}

type Dialer func(proto, addr string) (net.Conn, error)

type Status uint


