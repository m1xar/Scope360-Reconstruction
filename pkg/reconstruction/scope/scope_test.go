package scope

import "testing"

func TestHasAny(t *testing.T) {
	s := Closed | Balances
	if !s.Has(Closed) || s.Has(Open) || !s.Any(Open|Balances) || s.Any(Open|Fundings) {
		t.Fatalf("unexpected scope semantics for %b", s)
	}
	if !All.Has(Closed | Open | Balances | Transactions | Fundings) {
		t.Fatal("All must include every bit")
	}
}
