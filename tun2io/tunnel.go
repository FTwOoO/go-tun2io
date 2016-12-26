package tun2io

import (
	"fmt"
	"golang.org/x/net/context"
	"net"
	"sync"
	"time"
	"unsafe"
	"bytes"
)

/*

#include "device.c"

*/

import "C"

type TcpTunnel struct {
	Id         uint32
	tcpHandle  *tcpConnection

	destAddr   string
	connOut    net.Conn

	status     Status
	statusMu   sync.Mutex

	recvChunks chan []byte

	writing    sync.Mutex
	recvBuf    bytes.Buffer
	recvBufMu  sync.Mutex

	ctx        context.Context
	ctxCancel  context.CancelFunc
}

func NewTcpTunnel(client *C.tcp_pcb, dialFn Dialer, id uint32) (*TcpTunnel, error) {
	destAddr := C.dump_dest_addr(client)
	defer C.free(unsafe.Pointer(destAddr))

	t := &TcpTunnel{
		Id:id,
		tcpHandle:   &tcpConnection{pcb: client},
		destAddr: C.GoString(destAddr),
		recvChunks:    make(chan []byte, 256),
	}

	t.SetStatus(StatusConnecting)

	var err error
	if t.connOut, err = dialFn("tcp", t.destAddr); err != nil {
		t.SetStatus(StatusConnectionFailed)
		return nil, err
	}

	t.SetStatus(StatusConnected)

	return t, nil
}

func (t *TcpTunnel)Run() {

	t.ctx, t.ctxCancel = context.WithCancel(context.Background())

	writerOk := make(chan error)
	readerOk := make(chan error)

	go t.reader(readerOk)
	go t.writer(writerOk)

	<-writerOk
	<-readerOk

	t.SetStatus(StatusProxying)
}

func (t *TcpTunnel) SetStatus(s Status) {
	t.statusMu.Lock()
	t.status = s
	t.statusMu.Unlock()
}

func (t *TcpTunnel) Status() Status {
	t.statusMu.Lock()
	s := t.status
	t.statusMu.Unlock()
	return s
}

func (t *TcpTunnel) writeToClient() error {

	t.writing.Lock()
	defer t.writing.Unlock()

	// Sends tcp writes until tcp send buffer is full.
	for {

		t.recvBufMu.Lock()
		blen := uint(t.recvBuf.Len())
		t.recvBufMu.Unlock()

		if blen == 0 {
			return nil
		}

		mlen := t.tcpHandle.sndBufSize()
		if mlen == 0 {
			// At this point the actual tcp send buffer is full, let's wait for some
			// acks to try again.
			return errBufferIsFull
		}

		if blen > mlen {
			blen = mlen
		}

		chunk := make([]byte, blen)

		t.recvBufMu.Lock()
		if _, err := t.recvBuf.Read(chunk); err != nil {
			t.recvBufMu.Unlock()
			return err
		}
		t.recvBufMu.Unlock()

		// Enqueuing chunk.
		select {
		case t.recvChunks <- chunk:
		case <-t.ctx.Done():
			return t.ctx.Err()
		}
	}
}

func (t *TcpTunnel) writer(started chan error) error {
	started <- nil

	for {
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		case chunk := <-t.recvChunks:
			for i := 0; ; i++ {
				err := t.tcpHandle.tcpWrite(chunk)
				if err == nil {
					break
				}
				if err == errBufferIsFull {
					time.Sleep(time.Millisecond * 10)
					continue
				}
				return err
			}
		}
	}

	return nil
}

func (t *TcpTunnel) quit(reason string) error {
	status := t.Status()

	if status != StatusProxying {
		return fmt.Errorf("unexpected status %d", status)
	}

	t.SetStatus(StatusClosing)

	t.connOut.Close()

	t.SetStatus(StatusClosed)

	t.ctxCancel()

	return nil
}

// reader is the goroutine that reads whatever the connOut proxied destination
// receives and writes it to a buffer.
func (t *TcpTunnel) reader(started chan error) error {
	started <- nil

	for {
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		default:
			data := make([]byte, readBufSize)
			t.connOut.SetReadDeadline(time.Now().Add(ioTimeout))
			n, err := t.connOut.Read(data)
			if err != nil {
				return err
			}
			if n > 0 {
				t.recvBufMu.Lock()
				t.recvBuf.Write(data[0:n])
				t.recvBufMu.Unlock()

				go t.writeToClient()
			}
		}
	}

	return nil
}

