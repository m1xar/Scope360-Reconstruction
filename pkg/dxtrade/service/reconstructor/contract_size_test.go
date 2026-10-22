package reconstructor

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/service/reconstructor/helpers"
)

func readJSON(t *testing.T, name string, out any) {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func fixtures(t *testing.T) (map[string]float64, []models.Order, []models.Position) {
	t.Helper()
	var list models.InstrumentList
	var orders models.OrderList
	var positions struct {
		Positions []models.Position `json:"positions"`
	}
	readJSON(t, "instrument-list.json", &list)
	readJSON(t, "orders.json", &orders)
	readJSON(t, "positions.json", &positions)
	return helpers.LotSizes(list.Instruments), orders.Orders, positions.Positions
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

func TestContractSizeFromLotSize(t *testing.T) {
	lotSizes, _, _ := fixtures(t)
	cases := []struct {
		symbol string
		want   float64
	}{
		{"EURUSD", 100000}, // JSON number
		{"XAUUSD", 100},    // JSON string
		{"US30", 0},        // lotSize 0 -> nil
		{"GBPUSD", 0},      // absent
	}
	for _, c := range cases {
		wantSize(t, c.symbol, helpers.ContractSize(lotSizes, c.symbol), c.want)
	}
	wantSize(t, "nil map (instrument list 404)", helpers.ContractSize(nil, "EURUSD"), 0)
}

func TestBuildersContractSize(t *testing.T) {
	lotSizes, orders, open := fixtures(t)
	for _, hedging := range []bool{true, false} {
		positions := builders.BuildFXPositions(orders, nil, nil, nil, lotSizes, hedging)
		if len(positions) != 2 {
			t.Fatalf("hedging=%v: positions = %d, want 2", hedging, len(positions))
		}
		for _, p := range positions {
			switch p.Symbol {
			case "EURUSD":
				wantSize(t, "EURUSD", p.ContractSize, 100000)
				if p.Amount != 1.5 { // 150000 units / lotSize, unchanged behaviour
					t.Errorf("hedging=%v EURUSD Amount = %v, want 1.5", hedging, p.Amount)
				}
			case "US30":
				wantSize(t, "US30", p.ContractSize, 0)
				if p.Amount != 2 { // no lotSize: Lots() keeps units, nil flags it
					t.Errorf("hedging=%v US30 Amount = %v, want 2", hedging, p.Amount)
				}
			default:
				t.Errorf("unexpected symbol %q", p.Symbol)
			}
		}
	}

	openPositions := builders.BuildOpenPositions(open, nil, nil, nil, nil, lotSizes, true)
	if len(openPositions) != 1 {
		t.Fatalf("open = %d, want 1", len(openPositions))
	}
	wantSize(t, "open XAUUSD", openPositions[0].ContractSize, 100)
	if openPositions[0].Amount != 0.5 {
		t.Errorf("open Amount = %v, want 0.5", openPositions[0].Amount)
	}

	for _, p := range builders.BuildFXPositions(orders, nil, nil, nil, nil, true) {
		wantSize(t, p.Symbol+" without lot sizes", p.ContractSize, 0)
	}
}

func TestDatasetContractSizes(t *testing.T) {
	lotSizes, orders, open := fixtures(t)
	d := &Dataset{lotSizes: lotSizes, positions: open, orders: append(orders, models.Order{Instrument: "GBPJPY"})}
	specs, err := d.ContractSizes()
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		symbol string
		size   float64
	}{
		{"EURUSD", 100000},
		{"GBPJPY", 0}, // traded, no lotSize
		{"US30", 0},   // traded, lotSize 0
		{"XAUUSD", 100},
	}
	if len(specs) != len(want) {
		t.Fatalf("specs = %+v, want %d entries", specs, len(want))
	}
	for i, w := range want {
		s := specs[i]
		if s.Symbol != w.symbol || s.Pair != w.symbol || s.Source != domain.ContractSizeDXtrade {
			t.Errorf("spec[%d] = %+v, want %s", i, s, w.symbol)
		}
		wantSize(t, "spec "+w.symbol, s.ContractSize, w.size)
	}
}
