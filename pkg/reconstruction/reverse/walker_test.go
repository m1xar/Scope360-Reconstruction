package reverse

import "testing"

func TestResolvedAndPending(t *testing.T) {
	w := NewSeededWalker[string](map[string]float64{"BTC": 2})

	// Newest first: a close of an older episode, then the two opening fills
	// of the current position.
	if _, ok := w.Push("BTC", 1, "open-b"); ok {
		t.Fatal("position still open, no group expected")
	}
	if w.Resolved() {
		t.Fatal("not resolved after one opening fill")
	}
	if _, ok := w.Push("BTC", 1, "open-a"); ok {
		t.Fatal("the fills before an open position are not a closed group")
	}
	if !w.Resolved() {
		t.Fatal("expected resolved once the seeded size is walked back to zero")
	}
	if got := w.OpenFills()["BTC"]; len(got) != 2 || got[0] != "open-a" || got[1] != "open-b" {
		t.Fatalf("pending = %v, want [open-a open-b]", got)
	}

	// An older, fully closed episode is emitted and leaves nothing pending.
	w2 := NewWalker[string](nil)
	w2.Push("ETH", -1, "close")
	g, ok := w2.Push("ETH", 1, "open")
	if !ok || len(g.Fills) != 2 || g.Fills[0] != "open" {
		t.Fatalf("group = %+v ok=%v", g, ok)
	}
	if !w2.Resolved() || len(w2.OpenFills()) != 0 {
		t.Fatal("expected resolved with nothing pending")
	}
}
