package reconstructor

import (
	"os"
	"testing"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"google.golang.org/protobuf/encoding/protojson"
	gproto "google.golang.org/protobuf/proto"
)

// Fixtures are Open API responses in protojson form (the shape cTrader's JSON
// API returns; int64 as strings). Volumes and lotSize are in cents of units.
func readProto(t *testing.T, name string, out gproto.Message) {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := (protojson.UnmarshalOptions{AllowPartial: true}).Unmarshal(raw, out); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func symbolsFixture(t *testing.T) helpers.Symbols {
	t.Helper()
	var list pb.ProtoOASymbolsListRes
	var full pb.ProtoOASymbolByIdRes
	readProto(t, "symbols-list.json", &list)
	readProto(t, "symbol-by-id.json", &full)
	names := make(map[int64]string)
	for _, s := range list.GetSymbol() {
		names[s.GetSymbolId()] = s.GetSymbolName()
	}
	return helpers.NewSymbols(names, full.GetSymbol())
}

func wantSize(t *testing.T, what string, got *float64, want float64) {
	t.Helper()
	if want == 0 {
		if got != nil {
			t.Errorf("%s: ContractSize = %v, want nil", what, *got)
		}
		return
	}
	if got == nil || *got != want {
		t.Errorf("%s: ContractSize = %v, want %v", what, got, want)
	}
}

func TestSymbolsContractSizeIsLotSizeOver100(t *testing.T) {
	s := symbolsFixture(t)
	wantSize(t, "EURUSD lotSize 10000000", s.ContractSize(1), 100000)
	wantSize(t, "XAUUSD lotSize 10000", s.ContractSize(2), 100)
	wantSize(t, "US 30 without lotSize", s.ContractSize(3), 0)
	wantSize(t, "unknown id", s.ContractSize(99), 0)
	wantSize(t, "zero Symbols", helpers.Symbols{}.ContractSize(1), 0)
}

func TestBuildersContractSize(t *testing.T) {
	s := symbolsFixture(t)
	var deals pb.ProtoOADealListRes
	var reconcile pb.ProtoOAReconcileRes
	readProto(t, "deals.json", &deals)
	readProto(t, "reconcile.json", &reconcile)
	session := &connector.Session{MoneyDigits: 2, LeverageInCents: 10000}

	positions := builders.BuildFXPositions(deals.GetDeal(), nil, nil, s, session)
	if len(positions) != 2 {
		t.Fatalf("positions = %d, want 2", len(positions))
	}
	for _, p := range positions {
		switch p.ID {
		case "10":
			wantSize(t, "EURUSD", p.ContractSize, 100000)
			if p.Amount != 1.5 { // 15000000 cents / lotSize, unchanged
				t.Errorf("EURUSD Amount = %v, want 1.5", p.Amount)
			}
		case "20":
			wantSize(t, "US 30", p.ContractSize, 0)
			if p.Amount != 2 { // fallback volume/100 kept, nil flags it
				t.Errorf("US 30 Amount = %v, want 2", p.Amount)
			}
		default:
			t.Errorf("unexpected position %q", p.ID)
		}
	}

	open := builders.BuildOpenPositions(&reconcile, nil, nil, nil, s, session)
	if len(open) != 1 {
		t.Fatalf("open = %d, want 1", len(open))
	}
	wantSize(t, "open XAUUSD", open[0].ContractSize, 100)
	if open[0].Amount != 0.5 {
		t.Errorf("open Amount = %v, want 0.5", open[0].Amount)
	}
}

func TestSymbolSpecs(t *testing.T) {
	specs := symbolsFixture(t).SymbolSpecs()
	want := []struct {
		symbol, pair string
		size         float64
	}{
		{"EURUSD", "EURUSD", 100000},
		{"US 30", "US30", 0},
		{"XAUUSD", "XAUUSD", 100},
	}
	if len(specs) != len(want) {
		t.Fatalf("specs = %+v, want %d", specs, len(want))
	}
	for i, w := range want {
		s := specs[i]
		if s.Symbol != w.symbol || s.Pair != w.pair || s.Source != domain.ContractSizeCTrader {
			t.Errorf("spec[%d] = %+v, want %s/%s", i, s, w.symbol, w.pair)
		}
		wantSize(t, "spec "+w.symbol, s.ContractSize, w.size)
	}
}

func TestDatasetContractSizesNoFetchWhenLoaded(t *testing.T) {
	s := symbolsFixture(t)
	delete(s.Names, 3) // every remaining symbol already has its lotSize: no request, client is nil
	d := &Dataset{symbols: s}
	specs, err := d.ContractSizes()
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 || specs[0].Symbol != "EURUSD" || specs[1].Symbol != "XAUUSD" {
		t.Fatalf("specs = %+v", specs)
	}
	wantSize(t, "EURUSD", specs[0].ContractSize, 100000)
	again, _ := d.ContractSizes()
	if &again[0] != &specs[0] {
		t.Error("second call must return the cached catalog")
	}
}
