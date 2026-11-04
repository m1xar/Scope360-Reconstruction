package clocksync

import (
	"errors"
	"testing"
	"time"
)

func TestGetMeasuresOnceAndCaches(t *testing.T) {
	calls := 0
	o := New(func() (int64, error) {
		calls++
		return time.Now().UnixMilli() + 1500, nil
	})

	first := o.Get()
	second := o.Get()

	if calls != samples {
		t.Fatalf("fetch called %d times, want %d (one measurement)", calls, samples)
	}
	if first != second {
		t.Fatalf("cached offset changed: %d then %d", first, second)
	}
	if first < 1400 || first > 1600 {
		t.Fatalf("offset = %d, want about 1500", first)
	}
}

func TestMeasureKeepsShortestRoundTrip(t *testing.T) {
	// The slow sample reads the server clock late, which inflates its offset.
	delays := []time.Duration{40 * time.Millisecond, 0, 40 * time.Millisecond}
	call := 0
	o := New(func() (int64, error) {
		d := delays[call]
		call++
		time.Sleep(d)
		return time.Now().UnixMilli(), nil
	})

	if got := o.Get(); got < -5 || got > 5 {
		t.Fatalf("offset = %d, want the fast sample's ~0", got)
	}
}

func TestFailedMeasurementKeepsPreviousValue(t *testing.T) {
	fail := false
	o := New(func() (int64, error) {
		if fail {
			return 0, errors.New("down")
		}
		return time.Now().UnixMilli() + 1000, nil
	})
	before := o.Get()

	fail = true
	o.measuredAt = time.Now().Add(-time.Hour)
	if got := o.Get(); got != before {
		t.Fatalf("offset = %d after failed measurement, want previous %d", got, before)
	}
}

func TestRefreshIsRateLimited(t *testing.T) {
	calls := 0
	o := New(func() (int64, error) {
		calls++
		return time.Now().UnixMilli(), nil
	})
	o.Get()

	o.Refresh()
	if calls != samples {
		t.Fatalf("refresh right after a measurement re-measured: %d fetches", calls)
	}

	o.measuredAt = time.Now().Add(-minRefreshGap)
	o.Refresh()
	if calls != 2*samples {
		t.Fatalf("refresh after the gap did not re-measure: %d fetches", calls)
	}
}
