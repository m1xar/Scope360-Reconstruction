package reconstructor

import (
	"testing"

	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
)

const dayMs = int64(24 * 3600 * 1000)

func fill(tid int64, dir, side, sz, startPos string, atMs int64) models.RawFill {
	return models.RawFill{Coin: "ETH", Tid: tid, Dir: dir, Side: side, Sz: sz, StartPosition: startPos, Px: "1", Time: atMs}
}

func serve(all []models.RawFill, calls *int) func(startMs, endMs int64) ([]models.RawFill, error) {
	return func(startMs, endMs int64) ([]models.RawFill, error) {
		*calls++
		var out []models.RawFill
		for _, f := range all {
			if f.Time >= startMs && f.Time <= endMs {
				out = append(out, f)
			}
		}
		return out, nil
	}
}

func TestWalkEarlierStopsAtOpeningFill(t *testing.T) {
	now := int64(1_800_000_000_000)
	cutoff := now - 3*dayMs
	all := []models.RawFill{
		fill(1, "Open Long", "B", "2", "0", now-50*dayMs),  // opens the straddling position
		fill(2, "Close Long", "A", "2", "2", now-1*dayMs),  // closes it inside the window
		fill(3, "Open Long", "B", "1", "0", now-200*dayMs), // older, must never be fetched
		fill(4, "Close Long", "A", "1", "1", now-190*dayMs),
	}

	seg := helpers.NewFillSegmenter()
	seg.PushOlderBatch([]models.RawFill{all[1]}) // the cutoff window
	if seg.Flat() {
		t.Fatal("expected a half-walked position")
	}

	var calls int
	var synth int64
	earlier, segments, err := walkEarlier(serve(all, &calls), seg, cutoff-1, now-365*dayMs, &synth)
	if err != nil {
		t.Fatal(err)
	}
	if len(earlier) != 1 || earlier[0].Tid != 1 {
		t.Fatalf("earlier = %+v, want the opening fill only", earlier)
	}
	if len(segments) != 1 || len(segments[0]) != 2 {
		t.Fatalf("segments = %+v, want one closed episode of 2 fills", segments)
	}
	// 30-day windows: the opening fill 47 days before the cutoff sits in
	// the second window; the third window (with the old episode) is skipped.
	if calls != 2 {
		t.Errorf("fetch calls = %d, want 2", calls)
	}
}

func TestWalkEarlierNoopWhenFlat(t *testing.T) {
	var calls int
	var synth int64
	earlier, segments, err := walkEarlier(serve(nil, &calls), helpers.NewFillSegmenter(), 1000, 0, &synth)
	if err != nil || earlier != nil || segments != nil || calls != 0 {
		t.Fatalf("earlier=%v segments=%v calls=%d err=%v", earlier, segments, calls, err)
	}
}
