package reconstructor

import (
	"testing"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/binance/connector/binance/models"
)

const dayMs = int64(24 * 3600 * 1000)

func trade(id int64, side string, qty string, atMs int64) models.Trade {
	return models.Trade{Symbol: "BTCUSDT", ID: id, OrderID: id, Side: side, PositionSide: "BOTH", Qty: qty, Time: atMs}
}

// fetchFrom serves trades from a fixed set and records the windows asked for.
func fetchFrom(all []models.Trade, windows *int) func(startMs, endMs int64) ([]models.Trade, error) {
	return func(startMs, endMs int64) ([]models.Trade, error) {
		*windows++
		var out []models.Trade
		for _, t := range all {
			if t.Time >= startMs && t.Time <= endMs {
				out = append(out, t)
			}
		}
		return out, nil
	}
}

func TestWalkCompletesPositionStraddlingCutoff(t *testing.T) {
	now := time.Now().UnixMilli()
	all := []models.Trade{
		trade(1, "BUY", "1", now-40*dayMs),  // opens A (long before cutoff)
		trade(2, "SELL", "1", now-1*dayMs),  // closes A (inside window)
		trade(3, "BUY", "2", now-60*dayMs),  // opens B, closed before cutoff
		trade(4, "SELL", "2", now-55*dayMs), // closes B
	}
	cutoff := time.Now().Add(-3 * 24 * time.Hour)

	var windows int
	walk, err := walkSymbolFills(fetchFrom(all, &windows), "BTCUSDT", nil, &cutoff, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	groups := GroupsClosedAfter(walk.groups, &cutoff)
	if len(groups) != 1 || len(groups[0]) != 2 || groups[0][0].ID != 1 {
		t.Fatalf("groups = %+v, want the straddling position [1 2]", groups)
	}
	// Walked past the cutoff until fill 1 (6 weekly windows), but not on to
	// position B a further 3 weeks back.
	if windows < 6 || windows > 7 {
		t.Errorf("windows = %d, want 6-7", windows)
	}
	if len(walk.openFills) != 0 {
		t.Errorf("openFills = %+v, want none", walk.openFills)
	}
}

func TestWalkStopsRightAfterCutoffWhenFlat(t *testing.T) {
	now := time.Now().UnixMilli()
	all := []models.Trade{
		trade(1, "BUY", "1", now-2*dayMs),
		trade(2, "SELL", "1", now-1*dayMs),
		trade(3, "BUY", "1", now-100*dayMs), // old, never reached
		trade(4, "SELL", "1", now-90*dayMs),
	}
	cutoff := time.Now().Add(-3 * 24 * time.Hour)

	var windows int
	walk, err := walkSymbolFills(fetchFrom(all, &windows), "BTCUSDT", nil, &cutoff, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(walk.groups) != 1 {
		t.Fatalf("groups = %+v", walk.groups)
	}
	if windows != 1 {
		t.Errorf("windows = %d, want 1 (first window already covers the cutoff)", windows)
	}
}

func TestWalkResolvesOpenPosition(t *testing.T) {
	now := time.Now().UnixMilli()
	all := []models.Trade{
		trade(1, "BUY", "3", now-30*dayMs),  // opens the current position
		trade(2, "SELL", "1", now-20*dayMs), // partial close
		trade(3, "BUY", "5", now-80*dayMs),  // an older closed episode
		trade(4, "SELL", "5", now-70*dayMs),
	}
	open := []models.PositionRisk{{Symbol: "BTCUSDT", PositionSide: "BOTH", PositionAmt: "2"}}

	var windows int
	walk, err := walkSymbolFills(fetchFrom(all, &windows), "BTCUSDT", open, nil, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(walk.openFills) != 2 || walk.openFills[0].ID != 1 || walk.openFills[1].ID != 2 {
		t.Fatalf("openFills = %+v, want [1 2]", walk.openFills)
	}
	if windows > 5 {
		t.Errorf("windows = %d, want <= 5 (stop once resolved, before the old episode)", windows)
	}
	if len(walk.groups) != 0 {
		t.Errorf("groups = %+v, want none (walk ended before the old episode)", walk.groups)
	}
}

func TestWalkKeepsGoingForUntouchedOpenPosition(t *testing.T) {
	// An open position with no fills in the first windows must not be
	// reported as resolved: the seed keeps the walker non-flat.
	now := time.Now().UnixMilli()
	all := []models.Trade{trade(1, "BUY", "1", now-50*dayMs)}
	open := []models.PositionRisk{{Symbol: "BTCUSDT", PositionSide: "BOTH", PositionAmt: "1"}}

	var windows int
	walk, err := walkSymbolFills(fetchFrom(all, &windows), "BTCUSDT", open, nil, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(walk.openFills) != 1 || walk.openFills[0].ID != 1 {
		t.Fatalf("openFills = %+v, want [1]", walk.openFills)
	}
}

func TestWalkStopsAtSymbolFloor(t *testing.T) {
	// The open position cannot be resolved (its fills are missing); the
	// walk must not go past the symbol's first trade.
	now := time.Now().UnixMilli()
	all := []models.Trade{trade(1, "SELL", "1", now-20*dayMs)}
	open := []models.PositionRisk{{Symbol: "BTCUSDT", PositionSide: "BOTH", PositionAmt: "5"}}

	var windows int
	if _, err := walkSymbolFills(fetchFrom(all, &windows), "BTCUSDT", open, nil, true, now-20*dayMs-1); err != nil {
		t.Fatal(err)
	}
	if windows != 3 {
		t.Errorf("windows = %d, want 3 (20 days in 7-day windows)", windows)
	}
}
