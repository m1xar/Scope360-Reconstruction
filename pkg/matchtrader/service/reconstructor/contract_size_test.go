package reconstructor

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	connector "github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/service/reconstructor/helpers"
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

func fixtures(t *testing.T) (helpers.Instruments, []models.ClosedPosition, []models.OpenPosition) {
	t.Helper()
	var inst struct {
		Instruments []models.Instrument `json:"instruments"`
	}
	var open struct {
		Positions []models.OpenPosition `json:"positions"`
	}
	var closed []models.ClosedPosition
	readJSON(t, "effective-instruments.json", &inst)
	readJSON(t, "closed-positions.json", &closed)
	readJSON(t, "open-positions.json", &open)
	return helpers.IndexInstruments(inst.Instruments), closed, open.Positions
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

func TestInstrumentsContractSize(t *testing.T) {
	in, _, _ := fixtures(t)
	cases := []struct {
		symbol string
		want   float64 // 0 = nil
	}{
		{"EURUSD.r", 100000}, // JSON number
		{"XAUUSD.r", 100},    // JSON string
		{"BROKEN", 0},        // contractSize 0 -> not reported
		{"UNKNOWN", 0},       // not in the list
	}
	for _, c := range cases {
		wantSize(t, c.symbol, in.ContractSize(c.symbol), c.want)
	}
	wantSize(t, "nil index", helpers.Instruments(nil).ContractSize("EURUSD.r"), 0)
}

func TestBuildersContractSizeKeepsAmount(t *testing.T) {
	in, closed, open := fixtures(t)

	positions := builders.BuildPositions(closed, nil, 100)
	before := map[string]float64{}
	for _, p := range positions {
		before[p.ID] = p.Amount
		if p.ContractSize != nil {
			t.Fatalf("%s: builder must not set ContractSize by itself", p.ID)
		}
	}
	helpers.AttachContractSizes(positions, in)
	if len(positions) != 2 {
		t.Fatalf("positions = %d, want 2", len(positions))
	}
	for _, p := range positions {
		switch p.ID {
		case "P1":
			wantSize(t, "P1", p.ContractSize, 100000)
			if p.Amount != 1 {
				t.Errorf("P1 Amount = %v, want 1 lot", p.Amount)
			}
		case "P2":
			wantSize(t, "P2 (US30 without spec)", p.ContractSize, 0)
			if p.Amount != 2 {
				t.Errorf("P2 Amount = %v, want 2 lots", p.Amount)
			}
		}
		if p.Amount != before[p.ID] {
			t.Errorf("%s: Amount changed %v -> %v", p.ID, before[p.ID], p.Amount)
		}
	}

	openPositions := builders.BuildOpenPositions(open, nil)
	helpers.AttachOpenContractSizes(openPositions, in)
	if len(openPositions) != 1 {
		t.Fatalf("open = %d, want 1", len(openPositions))
	}
	wantSize(t, "open P3", openPositions[0].ContractSize, 100)
	if openPositions[0].Amount != 0.3 {
		t.Errorf("open Amount = %v, want 0.3", openPositions[0].Amount)
	}

	helpers.AttachContractSizes(positions, nil) // instruments failed to load
	for _, p := range positions {
		wantSize(t, p.ID+" without instruments", p.ContractSize, 0)
	}
}

func TestDatasetContractSizes(t *testing.T) {
	in, closed, open := fixtures(t)
	d := &Dataset{
		ctx:         context.Background(),
		now:         time.UnixMilli(1760100000000).UTC(),
		session:     &connector.Session{},
		balance:     &models.Balance{Balance: 10000},
		open:        open,
		closed:      closed,
		financeOK:   true,
		instruments: in,
	}

	// Closed positions built through the Dataset carry the size (scope 0: no candles).
	for _, p := range d.ClosedPositions() {
		want := map[string]float64{"P1": 100000, "P2": 0}[p.ID]
		wantSize(t, "dataset "+p.ID, p.ContractSize, want)
	}

	specs, err := d.ContractSizes() // instruments already loaded: no request, client is nil
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		symbol, pair string
		size         float64
	}{
		{"BROKEN", "BROKEN", 0},
		{"EURUSD.r", "EURUSD", 100000},
		{"US30", "US30", 0}, // traded but missing from the instrument list
		{"XAUUSD.r", "XAUUSD", 100},
	}
	if len(specs) != len(want) {
		t.Fatalf("specs = %+v, want %d entries", specs, len(want))
	}
	for i, w := range want {
		s := specs[i]
		if s.Symbol != w.symbol || s.Pair != w.pair || s.Source != domain.ContractSizeMatchTrader {
			t.Errorf("spec[%d] = %+v, want %s/%s", i, s, w.symbol, w.pair)
		}
		wantSize(t, "spec "+w.symbol, s.ContractSize, w.size)
	}
}
