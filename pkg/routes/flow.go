package routes

import (
	"sync/atomic"
	"time"
)

type Stats struct {
	LastSeen  time.Time
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
}

type Flow struct {
	timeout int64
	rxRoute *Route
	txRoute *Route

	lastSeen  int64
	rxBytes   uint64
	txBytes   uint64
	rxPackets uint64
	txPackets uint64
}

func (f *Flow) Stats() Stats {
	return Stats{
		LastSeen:  time.Unix(atomic.LoadInt64(&f.lastSeen), 0),
		RxBytes:   atomic.LoadUint64(&f.rxBytes),
		TxBytes:   atomic.LoadUint64(&f.txBytes),
		RxPackets: atomic.LoadUint64(&f.rxPackets),
		TxPackets: atomic.LoadUint64(&f.txPackets),
	}
}

func (f *Flow) Expired(now time.Time) bool {
	l := atomic.LoadInt64(&f.lastSeen)
	return l < (now.Unix() - f.timeout)
}

func (f *Flow) touch(now time.Time) {
	atomic.StoreInt64(&f.lastSeen, now.Unix())
}

func (f *Flow) receivedPacket(now time.Time, size uint64) {
	f.touch(now)
	atomic.AddUint64(&f.rxPackets, 1)
	atomic.AddUint64(&f.rxBytes, size)
}

func (f *Flow) sendPacket(now time.Time, size uint64) {
	f.touch(now)
	atomic.AddUint64(&f.txPackets, 1)
	atomic.AddUint64(&f.txBytes, size)
}
