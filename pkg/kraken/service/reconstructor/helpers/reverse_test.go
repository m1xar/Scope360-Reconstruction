package helpers

import (
	"testing"

	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

func fill(id, side string, size float64, at string) models.Fill {
	return models.Fill{FillID: id, Symbol: "PF_XBTUSD", Side: side, Size: models.FlexibleFloat(size), FillTime: at}
}

func TestSegmenterResolvesOpenPosition(t *testing.T) {
	open := []models.OpenPosition{{Symbol: "PF_XBTUSD", Side: "long", Size: models.FlexibleFloat(0.15)}}
	seg := NewFillSegmenter(open)
	if seg.Resolved() {
		t.Fatal("seeded position must not be resolved before any fill")
	}

	// Newest page: a partial exit and the two entries of the live position.
	seg.PushOlderBatch([]models.Fill{
		fill("c", "sell", 0.05, "2026-09-20T00:00:00.000Z"),
		fill("b", "buy", 0.10, "2026-09-15T00:00:00.000Z"),
	})
	if seg.Resolved() {
		t.Fatal("0.15 - (-0.05) - 0.10 = 0.10 still open")
	}
	seg.PushOlderBatch([]models.Fill{fill("a", "buy", 0.10, "2026-09-10T00:00:00.000Z")})
	if !seg.Resolved() {
		t.Fatal("expected resolved once the size is walked back to zero")
	}
	got := seg.OpenFills()["PF_XBTUSD"]
	if len(got) != 3 || got[0].FillID != "a" || got[2].FillID != "c" {
		t.Fatalf("open fills = %+v, want [a b c]", got)
	}
}
