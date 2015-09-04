package vmnet

import (
	"errors"
	"io"
	"net"
	"reflect"
	"sync"
	"unsafe"
)

// #cgo LDFLAGS: -framework vmnet
// #include "stddef.h"
// extern void VmnetOpen(void* goIface, void* msg);
// extern void VmnetClose(void* goIface, void* msg);
// extern void VmnetRead(void* goIface, void* msg);
// extern void VmnetWrite(void* goIface, void* msg);
import "C"

var (
	ErrGenericFailure                      = errors.New("vmnet: generic failure")
	ErrOutOfMemory                         = errors.New("vmnet: out of memory")
	ErrInvalidArgument                     = errors.New("vmnet: invalid argument")
	ErrInterfaceSetupIsNotComplete         = errors.New("vmnet: interface setup is not complete")
	ErrPermissionDenied                    = errors.New("vmnet: permission denied")
	ErrPacketSizeLargerThanMTU             = errors.New("vmnet: packet size larger than MTU")
	ErrBuffersExhaustedTemporarilyInKernel = errors.New("vmnet: buffers exhausted temporarily in kernel")
	ErrPacketsLargerThanLimit              = errors.New("vmnet: packets larger than limit")
	errNoMorePackets                       = errors.New("vmnet: no more packets")
	errNotWritten                          = errors.New("vmnet: packet not written")
)

var errCodeToErr = map[int]error{
	1001: ErrGenericFailure,
	1002: ErrOutOfMemory,
	1003: ErrInvalidArgument,
	1004: ErrInterfaceSetupIsNotComplete,
	1005: ErrPermissionDenied,
	1006: ErrPacketSizeLargerThanMTU,
	1007: ErrBuffersExhaustedTemporarilyInKernel,
	1008: ErrPacketsLargerThanLimit,

	// custom
	2000: errNoMorePackets,
	2001: errNotWritten,
}

type EventType uint32

const (
	packetsAvailableEvent EventType = 1 << 0
)

type cMsg struct {
	status   int
	buf      []byte
	pktSize  int
	pktFlags uint32
}

type Interface struct {
	mtx sync.RWMutex

	bufPool sync.Pool

	events chan Event
	readC  chan packet
	closed bool

	id            string
	macStr        string
	mac           net.HardwareAddr
	mtu           uint64
	maxPacketSize uint64
	iface         unsafe.Pointer
	eventQ        unsafe.Pointer
}

type Event struct {
	Type EventType
}

type packet struct {
	buf   []byte
	flags uint32
	err   error
}

func Open(uuid string) (*Interface, error) {
	var msg cMsg
	var iface = &Interface{
		id:     uuid,
		events: make(chan Event),
		readC:  make(chan packet),
	}

	C.VmnetOpen(unsafe.Pointer(iface), unsafe.Pointer(&msg))

	if msg.status != 1000 {
		err := errCodeToErr[msg.status]
		if err == nil {
			err = ErrGenericFailure
		}
		return nil, err
	}

	m, err := net.ParseMAC(iface.macStr)
	if err != nil {
		iface.Close()
		return nil, err
	}
	iface.mac = m

	return iface, nil
}

func (iface *Interface) ID() string {
	return iface.id
}

func (iface *Interface) MaxPacketSize() int {
	return int(iface.maxPacketSize)
}

func (iface *Interface) HardwareAddr() net.HardwareAddr {
	return iface.mac
}

func (iface *Interface) Close() error {
	if iface.iface == nil {
		return nil
	}

	iface.mtx.Lock()
	if iface.closed {
		iface.mtx.Unlock()
		return nil
	}
	iface.closed = true
	iface.mtx.Unlock()

	var msg cMsg
	C.VmnetClose(unsafe.Pointer(iface), unsafe.Pointer(&msg))

	close(iface.events)
	close(iface.readC)
	iface.iface = nil
	iface.eventQ = nil

	if msg.status != 1000 {
		err := errCodeToErr[msg.status]
		if err == nil {
			err = ErrGenericFailure
		}
		return err
	}

	return nil
}

func (iface *Interface) Events() <-chan Event {
	return iface.events
}

func (iface *Interface) ReadPacket(p []byte) (n int, flags uint32, err error) {
	if iface == nil {
		return 0, 0, io.EOF
	}
	if uint64(len(p)) < iface.maxPacketSize {
		return 0, 0, io.ErrShortBuffer
	}

	pkt, open := <-iface.readC
	if !open {
		return 0, 0, io.EOF
	}

	n = len(pkt.buf)
	if n > 0 {
		copy(p, pkt.buf)
	}
	if pkt.buf != nil {
		iface.bufPool.Put(pkt.buf)
		pkt.buf = nil
	}

	return n, pkt.flags, pkt.err
}

func (iface *Interface) WritePacket(p []byte, flags uint32) (n int, err error) {
	if iface == nil {
		return 0, io.EOF
	}
	if uint64(len(p)) > iface.maxPacketSize {
		return 0, io.ErrShortWrite
	}

	iface.mtx.RLock()
	defer iface.mtx.RUnlock()

	if iface.closed {
		return 0, io.EOF
	}

	var msg cMsg
	msg.buf = p
	msg.pktFlags = flags
	msg.pktSize = len(p)

	C.VmnetWrite(unsafe.Pointer(iface), unsafe.Pointer(&msg))

	if msg.status != 1000 {
		err := errCodeToErr[msg.status]
		if err == nil {
			err = ErrGenericFailure
		}
		return 0, err
	}

	return msg.pktSize, nil
}

func (iface *Interface) readPacket() (packet, error) {
	var pkt packet
	var msg cMsg

	if x, ok := iface.bufPool.Get().([]byte); x == nil || !ok {
		msg.buf = make([]byte, iface.maxPacketSize)
	} else {
		msg.buf = x[:cap(x)]
	}

	C.VmnetRead(unsafe.Pointer(iface), unsafe.Pointer(&msg))

	if msg.status != 1000 {
		err := errCodeToErr[msg.status]
		if err == nil {
			err = ErrGenericFailure
		}
		pkt.err = err
		return pkt, err
	}

	pkt.buf = msg.buf[:msg.pktSize]
	pkt.flags = msg.pktFlags
	return pkt, nil
}

//export cInterfaceGetIfaceRef
func cInterfaceGetIfaceRef(ptr unsafe.Pointer) unsafe.Pointer {
	if ptr == nil {
		return nil
	}

	iface := (*Interface)(ptr)
	return iface.iface
}

//export cInterfaceGetEventQueue
func cInterfaceGetEventQueue(ptr unsafe.Pointer) unsafe.Pointer {
	if ptr == nil {
		return nil
	}

	iface := (*Interface)(ptr)
	return iface.eventQ
}

//export cInterfaceGetID
func cInterfaceGetID(ptr unsafe.Pointer) *C.char {
	iface := (*Interface)(ptr)
	if iface.id == "" {
		return nil
	}
	return C.CString(iface.id)
}

//export cInterfaceSetID
func cInterfaceSetID(ptr unsafe.Pointer, x *C.char) {
	iface := (*Interface)(ptr)
	iface.id = C.GoString(x)
}

//export cInterfaceSetMAC
func cInterfaceSetMAC(ptr unsafe.Pointer, x *C.char) {
	iface := (*Interface)(ptr)
	iface.macStr = C.GoString(x)
}

//export cInterfaceSetMTU
func cInterfaceSetMTU(ptr unsafe.Pointer, x uint64) {
	iface := (*Interface)(ptr)
	iface.mtu = x
}

//export cInterfaceSetMaxPacketSize
func cInterfaceSetMaxPacketSize(ptr unsafe.Pointer, x uint64) {
	iface := (*Interface)(ptr)
	iface.maxPacketSize = x
}

//export cInterfaceSetIfaceRef
func cInterfaceSetIfaceRef(ptr unsafe.Pointer, x unsafe.Pointer) {
	iface := (*Interface)(ptr)
	iface.iface = x
}

//export cInterfaceSetEventQueue
func cInterfaceSetEventQueue(ptr unsafe.Pointer, x unsafe.Pointer) {
	iface := (*Interface)(ptr)
	iface.eventQ = x
}

//export cInterfaceEmitEvent
func cInterfaceEmitEvent(ptr unsafe.Pointer, eventType uint32, nPktAvail uint64) {
	iface := (*Interface)(ptr)
	etype := EventType(eventType)

	switch etype {
	case packetsAvailableEvent:
		for i := 0; i < int(nPktAvail); i++ {
			pkt, err := iface.readPacket()
			if err == errNoMorePackets {
				break
			}

			iface.readC <- pkt
			if err != nil {
				break
			}
		}
	default:
		iface.events <- Event{Type: etype}
	}
}

//export cMsgSetStatus
func cMsgSetStatus(ptr unsafe.Pointer, x int) {
	msg := (*cMsg)(ptr)
	msg.status = x
}

//export cMsgGetBufPtr
func cMsgGetBufPtr(ptr unsafe.Pointer) unsafe.Pointer {
	msg := (*cMsg)(ptr)
	hdrp := (*reflect.SliceHeader)(unsafe.Pointer(&msg.buf))
	return unsafe.Pointer(hdrp.Data)
}

//export cMsgGetBufLen
func cMsgGetBufLen(ptr unsafe.Pointer) C.size_t {
	msg := (*cMsg)(ptr)
	return C.size_t(len(msg.buf))
}

//export cMsgSetPacketSize
func cMsgSetPacketSize(ptr unsafe.Pointer, x C.size_t) {
	msg := (*cMsg)(ptr)
	msg.pktSize = int(x)
}

//export cMsgSetPacketFlags
func cMsgSetPacketFlags(ptr unsafe.Pointer, x uint32) {
	msg := (*cMsg)(ptr)
	msg.pktFlags = x
}

//export cMsgGetPacketFlags
func cMsgGetPacketFlags(ptr unsafe.Pointer) uint32 {
	msg := (*cMsg)(ptr)
	return msg.pktFlags
}
