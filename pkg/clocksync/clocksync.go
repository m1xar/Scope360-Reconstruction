// Package clocksync keeps one estimate of an exchange's clock offset per process.
//
// The offset is a property of the host clock, not of a client or an account, so it
// is measured once and shared: measuring it per client adds a round trip to every
// sync, and a single sample taken under load can be off by more than the exchange's
// tolerance when the request and response legs are delayed unevenly.
package clocksync

import (
	"sync"
	"time"
)

const (
	samples = 3
	// ttl bounds how far the host clock may drift before the offset is re-measured.
	ttl = 10 * time.Minute
	// minRefreshGap stops a burst of timestamp rejections from re-measuring once each.
	minRefreshGap = 5 * time.Second
	// maxSampleRTT drops samples whose round trip makes the midpoint estimate useless.
	maxSampleRTT = 2 * time.Second
)

// Offset is the exchange time minus the local time, in milliseconds.
type Offset struct {
	fetch func() (serverMs int64, err error)

	mu         sync.Mutex
	value      int64
	measuredAt time.Time
}

// New returns an Offset that measures with fetch, which returns the exchange time in
// Unix milliseconds.
func New(fetch func() (serverMs int64, err error)) *Offset {
	return &Offset{fetch: fetch}
}

// Get returns the offset, measuring it first when it is missing or older than ttl.
func (o *Offset) Get() int64 {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.measuredAt.IsZero() || time.Since(o.measuredAt) > ttl {
		o.measureLocked()
	}
	return o.value
}

// Refresh re-measures the offset after the exchange rejected a timestamp.
func (o *Offset) Refresh() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if time.Since(o.measuredAt) < minRefreshGap {
		return
	}
	o.measureLocked()
}

// measureLocked keeps the sample with the shortest round trip, whose midpoint is the
// closest to the moment the exchange read its clock. When every sample fails the
// previous value stays, and the next Get retries after ttl.
func (o *Offset) measureLocked() {
	o.measuredAt = time.Now()

	bestRTT := time.Duration(-1)
	for range samples {
		before := time.Now()
		serverMs, err := o.fetch()
		rtt := time.Since(before)
		if err != nil || serverMs == 0 || rtt > maxSampleRTT {
			continue
		}
		if bestRTT < 0 || rtt < bestRTT {
			bestRTT = rtt
			o.value = serverMs - before.Add(rtt/2).UnixMilli()
		}
	}
}
