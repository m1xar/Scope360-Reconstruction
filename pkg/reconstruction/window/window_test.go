package window

import (
	"testing"
	"time"
)

func TestBackward(t *testing.T) {
	spans := Backward(100, 0, 40)
	want := []Span{{61, 100}, {21, 60}, {1, 20}}
	if len(spans) != len(want) {
		t.Fatalf("got %v", spans)
	}
	for i := range want {
		if spans[i] != want[i] {
			t.Fatalf("span %d = %v, want %v", i, spans[i], want[i])
		}
	}
	if Backward(10, 10, 5) != nil || Backward(10, 0, 0) != nil {
		t.Fatal("expected nil for empty ranges")
	}
}

func TestDaysSinceAndStartMs(t *testing.T) {
	if got := DaysSince(time.Now().Add(-36 * time.Hour)); got != 3 {
		t.Errorf("DaysSince(36h ago) = %d, want 3 (2 days rounded up + 1 slack)", got)
	}
	if got := DaysSince(time.Now()); got < 1 || got > 2 {
		t.Errorf("DaysSince(now) = %d, want 1 or 2", got)
	}
	if StartMs(nil) != 0 {
		t.Error("StartMs(nil) must be 0")
	}
	at := time.UnixMilli(1_700_000_000_000)
	if StartMs(&at) != 1_700_000_000_000 {
		t.Error("StartMs must return the cutoff in ms")
	}
}
