// Package window slices a time range into fixed spans for walking exchange
// history from the newest records to the oldest.
package window

import "time"

const (
	Day       = 24 * time.Hour
	Retention = 3 * 365 * Day
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
