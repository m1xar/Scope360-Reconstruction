package excursion

import (
	"testing"

	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/candlespan"
)

func ptr(v float64) *float64 { return &v }

func TestInsideDropsBarsStraddlingEntryAndExit(t *testing.T) {
	open := int64(10*minuteMs + 30_000)  // 00:10:30
	close := int64(20*minuteMs + 40_000) // 00:20:40

	cases := []struct {
		name  string
		barMs int64
		at    int64
		want  bool
	}{
		{"entry bar", minuteMs, 10 * minuteMs, false},
		{"first full bar", minuteMs, 11 * minuteMs, true},
		{"last full bar", minuteMs, 19 * minuteMs, true},
		{"exit bar", minuteMs, 20 * minuteMs, false},
		{"unknown interval", 0, 15 * minuteMs, false},
	}
	for _, c := range cases {
		if got := Inside(c.at, c.barMs, open, close); got != c.want {
			t.Errorf("%s: Inside = %v, want %v", c.name, got, c.want)
		}
	}

	if !Inside(0, minuteMs, 0, minuteMs) {
		t.Error("bar exactly matching the position must be kept")
	}
}

func TestInsideKeepsUTCDaysOfLongPositions(t *testing.T) {
	open := int64(dayMs + 5*60*minuteMs)  // day 1, 05:00
	close := int64(4*dayMs + 30*minuteMs) // day 4, 00:30
	for _, seg := range candlespan.Split(open, close) {
		if seg.Interval != candlespan.Day {
			continue
		}
		for at := seg.StartMs; at < seg.EndMs; at += dayMs {
			if !Inside(at, IntervalMs(seg.Interval), open, close) {
				t.Errorf("daily bar at %d dropped", at)
			}
		}
	}
	// A daily bar aligned to UTC+8 (16:00 UTC) sticks out past the exit.
	if Inside(3*dayMs+16*60*minuteMs, dayMs, open, close) {
		t.Error("misaligned daily bar crossing the exit must be dropped")
	}
}

func TestComputeLongStopLoss(t *testing.T) {
	// 1000 SOL long, stopped out 1.846 below entry; inner bars never went
	// below the stop, so MAE is the realised net loss.
	gross, net := -1846.45, -1911.57
	mae, mfe := Compute("LONG", 100, 1000, ptr(102.25), ptr(98.5), gross, net)
	if *mae != net {
		t.Errorf("MAE = %v, want %v", *mae, net)
	}
	if want := 2250 + (net - gross); *mfe != round8(want) {
		t.Errorf("MFE = %v, want %v", *mfe, want)
	}
}

func TestComputeKeepsDeeperDrawdown(t *testing.T) {
	// Price dipped 3 below entry and recovered to a small win.
	mae, mfe := Compute("LONG", 100, 10, ptr(101), ptr(97), 10, 8)
	if *mae != -32 {
		t.Errorf("MAE = %v, want -32", *mae)
	}
	if *mfe != 8 {
		t.Errorf("MFE = %v, want 8 (clamped up to net PnL)", *mfe)
	}
}

func TestComputeShort(t *testing.T) {
	mae, mfe := Compute("SHORT", 100, 10, ptr(102), ptr(95), 30, 28)
	if *mae != -22 {
		t.Errorf("MAE = %v, want -22", *mae)
	}
	if *mfe != 48 {
		t.Errorf("MFE = %v, want 48", *mfe)
	}
}

func TestComputeNeverCrossesZero(t *testing.T) {
	// Price never went against the long: MAE is 0 at most, MFE at least 0.
	mae, mfe := Compute("LONG", 100, 10, ptr(105), ptr(100.5), 40, 40)
	if *mae != 0 {
		t.Errorf("MAE = %v, want 0", *mae)
	}
	if *mfe != 50 {
		t.Errorf("MFE = %v, want 50", *mfe)
	}

	mae, mfe = Compute("LONG", 100, 10, ptr(99.9), ptr(95), -45, -45)
	if *mfe != 0 {
		t.Errorf("losing trade MFE = %v, want 0", *mfe)
	}
	if *mae != -50 {
		t.Errorf("losing trade MAE = %v, want -50", *mae)
	}
}

func TestComputeWithoutInnerBars(t *testing.T) {
	mae, mfe := Compute("LONG", 100, 10, nil, nil, -20, -22)
	if *mae != -22 || *mfe != 0 {
		t.Errorf("loss: MAE, MFE = %v, %v; want -22, 0", *mae, *mfe)
	}

	mae, mfe = Compute("SHORT", 100, 10, nil, nil, 20, 18)
	if *mae != -2 || *mfe != 18 {
		t.Errorf("win: MAE, MFE = %v, %v; want -2, 18", *mae, *mfe)
	}
}
