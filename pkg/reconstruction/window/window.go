// Package window slices a time range into fixed spans for walking exchange
// history from the newest records to the oldest.
package window

import (
	"math"
	"time"
)

const (
	day       = 24 * time.Hour
	Retention = 3 * 365 * day
)

type Span struct {
	StartMs int64
	EndMs   int64
}

// Backward returns contiguous spans of spanMs covering (floorMs, endMs],
// newest first. The last span is clamped to floorMs.
func Backward(endMs, floorMs, spanMs int64) []Span {
	if spanMs <= 0 || endMs <= floorMs {
		return nil
	}
	var spans []Span
	for end := endMs; end > floorMs; end -= spanMs {
		start := end - spanMs + 1
		if start <= floorMs {
			start = floorMs + 1
		}
		spans = append(spans, Span{StartMs: start, EndMs: end})
	}
	return spans
}

// DaysSince returns the days window that reaches back to t plus one day of
// slack, for lookups of a position known to have opened at t.
func DaysSince(t time.Time) int {
	return int(math.Ceil(time.Since(t).Hours()/24)) + 1
}

// StartMs converts an optional cutoff into the millisecond start used by
// the exchange fetchers, where 0 means unbounded.
func StartMs(cutoff *time.Time) int64 {
	if cutoff == nil {
		return 0
	}
	return cutoff.UnixMilli()
}
