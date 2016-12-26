package tun2io

import (
	"fmt"
	"sync"
	"unsafe"
)

/*
#include "device.c"
*/

import "C"

type tcpConnection struct {
	localAddr  *Addr
	remoteAddr *Addr

	pcb        *C.tcp_pcb
	pcbMu      sync.Mutex
}

func (t *tcpConnection) tcpWrite(chunk []byte) error {
	clen := len(chunk)
	cchunk := C.CString(string(chunk))
	defer C.free(unsafe.Pointer(cchunk))

	t.pcbMu.Lock()
	err_t := C.tcp_write(t.pcb, unsafe.Pointer(cchunk), C.uint16_t(clen), C.TCP_WRITE_FLAG_COPY)
	t.pcbMu.Unlock()

	switch err_t {
	case C.ERR_OK:
		return nil
	case C.ERR_MEM:
		return errBufferIsFull
	}

	return fmt.Errorf("C.tcp_write: %d", int(err_t))
}

func (t *tcpConnection) sndBufSize() uint {

	t.pcbMu.Lock()
	s := C.tcp_sndbuf(t.pcb)
	t.pcbMu.Unlock()

	return uint(s)
}

func (t *tcpConnection) tcpOutput() error {
	t.pcbMu.Lock()
	err_t := C.tcp_output(t.pcb)
	t.pcbMu.Unlock()

	if err_t != C.ERR_OK {
		return fmt.Errorf("C.tcp_output: %d", int(err_t))
	}

	return nil
}
