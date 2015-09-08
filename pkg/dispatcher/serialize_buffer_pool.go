package dispatcher

import (
	"sync"

	"github.com/google/gopacket"
)

var serializeBufferPool sync.Pool

func getSerializeBuffer() gopacket.SerializeBuffer {
	buf, _ := serializeBufferPool.Get().(gopacket.SerializeBuffer)
	if buf != nil {
		return buf
	}
	return gopacket.NewSerializeBuffer()
}

func putSerializeBuffer(buf gopacket.SerializeBuffer) {
	if buf == nil {
		return
	}

	buf.Clear()
	serializeBufferPool.Put(buf)
}
