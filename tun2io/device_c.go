package tun2io

import (
	"log"
	"math/rand"
	"time"
	"net"
	"unsafe"
)

/*

#cgo CFLAGS: -DLWIP_DONT_PROVIDE_BYTEORDER_FUNCTIONS -DCGO=1 -I${SRCDIR} -I${SRCDIR}/.. -I${SRCDIR}/../lwip/src/include -I${SRCDIR}/../lwip-contrib/ports/unix/port/include

#cgo LDFLAGS: -L${SRCDIR}/../lib/ -llwip

#include "device.h"

*/
import "C"

var device *Device

func goSetDevice(d *Device) {
	device = d
}

func cInitLwip(addr net.IP, mask net.IPMask) {
	C.lwip_initialize(C.CString(addr.String()), C.CString(mask.String()))
	go func() {
		tk := time.NewTicker(time.Duration(C.TCP_TMR_INTERVAL))
		for {
			select {
			case <-tk.C:
				C.tcp_tmr()
			}
		}
	}()

}


func cLwipWrite(b []byte) {
	data := (*C.char)(unsafe.Pointer(&b[0]))
	datalen := C.uint(len(data))
	C.lwip_input(data, datalen)
}

func cLwipDestrop() {
	C.lwip_detrop()
}

func cStringToGoString(data *C.char, len C.size_t) []byte {
	size := int(len)
	buf := make([]byte, size)

	for i := 0; i < size; i++ {
		buf[i] = byte(C.charAt(data, C.int(i)))
	}

	return buf
}


func goNewTunnel(client *C.struct_tcp_pcb) C.uint32_t {
	device.TunnelMu.Lock()
	defer device.TunnelMu.Unlock()

	var i uint32
	for {
		i = uint32(rand.Int31())
		if _, ok := device.Tunnels[i]; !ok {

			t, err := NewTcpTunnel(client, device.Dialer(), i)
			if err != nil {
				log.Printf("Could not start tunnel: %q", err)
				return 0
			}

			device.Tunnels[i] = t
			t.Run()
			return C.uint32_t(i)
		}
	}

	panic("reached.")
}


func goTunnelWrite(tunno C.uint32_t, write *C.char, size C.size_t) C.int {
	device.TunnelMu.Lock()
	t, ok := device.Tunnels[uint32(tunno)]
	device.TunnelMu.Unlock()

	if ok {
		buf := cStringToGoString(write, size)

		if t.Status() != StatusProxying {
			return C.ERR_ABRT
		}

		t.connOut.SetWriteDeadline(time.Now().Add(ioTimeout))
		if _, err := t.connOut.Write(buf); err == nil {
			return C.ERR_OK
		}
	}

	return C.ERR_ABRT
}

func goLwipOutPacket(write *C.char, size C.size_t) {
	buf := cStringToGoString(write, size)
	device.readChan <- buf
}

func goTunnelSentACK(tunno C.uint32_t, dlen C.u16_t) C.int {
	tunID := uint32(tunno)

	device.TunnelMu.Lock()
	t, ok := device.Tunnels[tunID]
	device.TunnelMu.Unlock()

	if !ok {
		return C.ERR_ABRT
	}

	// Now that the client ACKed a few packages we might be able to continue
	// writing.
	go t.writeToClient()

	return C.ERR_OK
}

func goTunnelDestroy(tunno C.uint32_t) C.int {
	tunID := uint32(tunno)

	device.TunnelMu.Lock()
	t, ok := device.Tunnels[tunID]
	device.TunnelMu.Unlock()

	if !ok {
		return C.ERR_ABRT
	}

	if err := t.quit(reasonClientAbort); err != nil {
		return C.ERR_ABRT
	}

	device.TunnelMu.Lock()
	delete(device.Tunnels, tunID)
	device.TunnelMu.Unlock()

	return C.ERR_OK
}

func goLog(level C.int, c *C.char) {
	s := C.GoString(c)
	log.Printf("tun2io: %s", s)
	return
}