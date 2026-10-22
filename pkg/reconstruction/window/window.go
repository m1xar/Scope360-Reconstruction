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

func DaysSince(t time.Time) int {
	return int(math.Ceil(time.Since(t).Hours()/24)) + 1
}

func StartMs(cutoff *time.Time) int64 {
	if cutoff == nil {
		return 0
	}
	return cutoff.UnixMilli()
}
