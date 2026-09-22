package reconstructor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
)

// fakeOKX serves one closed BTC position, one open ETH position and a
// handful of bills, filtering by instType, type and begin/end like the real
// archive does, and counts the requests per path.
type fakeOKX struct {
	*httptest.Server
	mu   sync.Mutex
	hits map[string]int
	now  int64
}

const hour = int64(3600 * 1000)

func newFakeOKX(t *testing.T) *fakeOKX {
	f := &fakeOKX{hits: map[string]int{}, now: time.Now().UnixMilli()}
	day := 24 * hour
	btcOpen, btcClose := f.now-3*day, f.now-1*day
	ethOpen := f.now - 2*day

	row := func(kv ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = fmt.Sprint(kv[i+1])
		}
		return m
	}
	closed := []map[string]any{row("instId", "BTC-USDT-SWAP", "instType", "SWAP", "mgnMode", "cross", "posId", "1",
		"direction", "long", "openAvgPx", 100, "closeAvgPx", 110, "openMaxPos", 2, "closeTotalPos", 2,
		"pnl", 20, "realizedPnl", 18.5, "fee", -1, "fundingFee", -0.5, "lever", 10, "cTime", btcOpen, "uTime", btcClose)}
	open := []map[string]any{row("instId", "ETH-USDT-SWAP", "instType", "SWAP", "mgnMode", "cross", "posId", "2",
		"posSide", "net", "pos", 3, "avgPx", 50, "markPx", 55, "lever", 5, "cTime", ethOpen)}
	orders := []map[string]any{
		row("instId", "BTC-USDT-SWAP", "instType", "SWAP", "ordId", "o1", "ordType", "market", "side", "buy", "posSide", "net",
			"sz", 2, "accFillSz", 2, "avgPx", 100, "state", "filled", "fee", -0.5, "tdMode", "cross", "cTime", btcOpen, "uTime", btcOpen),
		row("instId", "BTC-USDT-SWAP", "instType", "SWAP", "ordId", "o2", "ordType", "market", "side", "sell", "posSide", "net",
			"sz", 2, "accFillSz", 2, "avgPx", 110, "state", "filled", "fee", -0.5, "pnl", 20, "tdMode", "cross", "cTime", btcClose, "uTime", btcClose),
		row("instId", "ETH-USDT-SWAP", "instType", "SWAP", "ordId", "o3", "ordType", "limit", "side", "buy", "posSide", "net",
			"sz", 3, "accFillSz", 3, "avgPx", 50, "state", "filled", "fee", -0.3, "tdMode", "cross", "cTime", ethOpen-hour, "uTime", ethOpen),
	}
	fills := []map[string]any{
		row("instId", "BTC-USDT-SWAP", "instType", "SWAP", "ordId", "o1", "tradeId", "t1", "billId", "b1", "side", "buy", "posSide", "net", "fillSz", 2, "fillPx", 100, "fee", -0.5, "ts", btcOpen),
		row("instId", "BTC-USDT-SWAP", "instType", "SWAP", "ordId", "o2", "tradeId", "t2", "billId", "b2", "side", "sell", "posSide", "net", "fillSz", 2, "fillPx", 110, "fee", -0.5, "fillPnl", 20, "ts", btcClose),
		row("instId", "ETH-USDT-SWAP", "instType", "SWAP", "ordId", "o3", "tradeId", "t3", "billId", "b3", "side", "buy", "posSide", "net", "fillSz", 3, "fillPx", 50, "fee", -0.3, "ts", ethOpen),
	}
	bills := []map[string]any{
		row("billId", "10", "instId", "BTC-USDT-SWAP", "instType", "SWAP", "type", "2", "balChg", 20, "bal", 1020, "ccy", "USDT", "ts", btcClose),
		row("billId", "11", "instId", "ETH-USDT-SWAP", "instType", "SWAP", "type", "8", "balChg", -0.25, "bal", 1019.75, "ccy", "USDT", "ts", f.now-12*hour),
		row("billId", "12", "instId", "", "instType", "", "type", "1", "from", "6", "to", "18", "balChg", 500, "bal", 1000, "ccy", "USDT", "ts", f.now-40*hour),
		row("billId", "13", "instId", "BTC-USDT", "instType", "SPOT", "type", "2", "balChg", -3, "bal", 997, "ccy", "USDT", "ts", f.now-30*hour),
		row("billId", "14", "instId", "ETH-USDT-SWAP", "instType", "SWAP", "type", "8", "balChg", -0.1, "bal", 999.9, "ccy", "USDT", "ts", btcOpen-5*60*1000),
	}
	instruments := map[string]map[string]any{
		"BTC-USDT-SWAP": row("instId", "BTC-USDT-SWAP", "instType", "SWAP", "ctVal", 1, "ctMult", 1),
		"ETH-USDT-SWAP": row("instId", "ETH-USDT-SWAP", "instType", "SWAP", "ctVal", 1, "ctMult", 1),
	}

	inRange := func(q map[string][]string, ts string) bool {
		t, _ := strconv.ParseInt(ts, 10, 64)
		if b := q["begin"]; len(b) > 0 {
			if begin, _ := strconv.ParseInt(b[0], 10, 64); t < begin {
				return false
			}
		}
		if e := q["end"]; len(e) > 0 {
			if end, _ := strconv.ParseInt(e[0], 10, 64); t > end {
				return false
			}
		}
		return true
	}
	filter := func(rows []map[string]any, q map[string][]string, tsField string) []map[string]any {
		var out []map[string]any
		for _, r := range rows {
			if v := q["instType"]; len(v) > 0 && v[0] != "" && r["instType"] != v[0] {
				continue
			}
			if v := q["type"]; len(v) > 0 && v[0] != "" && r["type"] != v[0] {
				continue
			}
			if v := q["after"]; len(v) > 0 && v[0] != "" {
				continue // single page
			}
			if tsField != "" && !inRange(q, r[tsField].(string)) {
				continue
			}
			out = append(out, r)
		}
		return out
	}

	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits[r.URL.Path]++
		f.mu.Unlock()
		q := r.URL.Query()
		var data any
		switch {
		case strings.HasSuffix(r.URL.Path, "/account/positions-history"):
			data = filter(closed, q, "")
		case strings.HasSuffix(r.URL.Path, "/account/positions"):
			data = filter(open, q, "")
		case strings.HasSuffix(r.URL.Path, "/account/balance"):
			data = []map[string]any{{"totalEq": "1019.75"}}
		case strings.HasSuffix(r.URL.Path, "/account/bills-archive"):
			data = filter(bills, q, "ts")
		case strings.HasSuffix(r.URL.Path, "/trade/orders-history-archive"):
			data = filter(orders, q, "cTime")
		case strings.HasSuffix(r.URL.Path, "/trade/fills-history"):
			data = filter(fills, q, "ts")
		case strings.HasSuffix(r.URL.Path, "/public/instruments"):
			data = []map[string]any{instruments[q.Get("instId")]}
		case strings.HasSuffix(r.URL.Path, "/market/history-candles"):
			after, _ := strconv.ParseInt(q.Get("after"), 10, 64)
			ts := after - 2*hour
			data = [][]string{{fmt.Sprint(ts), "105", "108", "95", "104", "1", "1"}}
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		body, _ := json.Marshal(map[string]any{"code": "0", "data": data})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	return f
}

func (f *fakeOKX) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hits = map[string]int{}
}

func (f *fakeOKX) count(suffix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	for path, n := range f.hits {
		if strings.HasSuffix(path, suffix) {
			return n
		}
	}
	return 0
}

func TestLoadAllFetchesEveryDatasetOnce(t *testing.T) {
	f := newFakeOKX(t)
	defer f.Close()
	client := resty.New()
	cutoff := time.Now().Add(-60 * time.Hour) // inside the 3-day-old BTC position

	d, err := Load(client, f.URL, &cutoff, scope.All)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"/account/balance":              1,
		"/account/positions":            2, // SWAP + FUTURES
		"/account/positions-history":    2, // SWAP + FUTURES
		"/account/bills-archive":        1, // every instType in one stream
		"/trade/orders-history-archive": 2,
		"/trade/fills-history":          2,
		"/public/instruments":           2, // one per instId
	}
	for suffix, n := range want {
		if got := f.count(suffix); got != n {
			t.Errorf("%s: %d requests, want %d", suffix, got, n)
		}
	}

	positions, err := d.ClosedPositions()
	if err != nil || len(positions) != 1 {
		t.Fatalf("closed positions = %d, err %v", len(positions), err)
	}
	p := positions[0]
	if p.Pair != "BTCUSDT" || p.Amount != 2 || p.NetPnl != 18.5 || len(p.Orders) != 2 {
		t.Errorf("unexpected position %+v", p)
	}
	if p.MAE == nil || p.MFE == nil || *p.MAE != -11.5 || *p.MFE != 18.5 {
		t.Errorf("MAE/MFE = %v/%v, want -11.5/18.5", p.MAE, p.MFE)
	}
	if p.BalanceInit != 999.9 {
		t.Errorf("BalanceInit = %v, want 999.9 (last SWAP bill before open)", p.BalanceInit)
	}
	open, err := d.OpenPositions()
	if err != nil || len(open) != 1 || len(open[0].Orders) != 1 || open[0].Amount != 3 {
		t.Fatalf("open positions = %+v, err %v", open, err)
	}
	if snaps := d.BalanceSnapshots(); len(snaps) != 3 { // bill 10, bill 11 and now; SPOT and pre-window rows excluded
		t.Errorf("snapshots = %+v, want 3", snaps)
	}
	if txs := d.Transactions(); len(txs) != 1 || txs[0].Type != domain.TransactionTypeDeposit || txs[0].Amount != 500 {
		t.Errorf("transactions = %+v", txs)
	}
	if fundings := d.Fundings(); len(fundings) != 1 || fundings[0].Amount != -0.25 {
		t.Errorf("fundings = %+v, want the one inside the window", fundings)
	}
	if bal := d.CurrentBalance(); bal != 1019.75 {
		t.Errorf("balance = %v", bal)
	}
}

func TestScopedLoadsMatchTheFullOne(t *testing.T) {
	f := newFakeOKX(t)
	defer f.Close()
	client := resty.New()
	cutoff := time.Now().Add(-60 * time.Hour) // inside the 3-day-old BTC position

	all, err := Load(client, f.URL, &cutoff, scope.All)
	if err != nil {
		t.Fatal(err)
	}
	fullClosed, _ := all.ClosedPositions()
	fullOpen, _ := all.OpenPositions()

	f.reset()
	closedOnly, err := Load(client, f.URL, &cutoff, scope.Closed)
	if err != nil {
		t.Fatal(err)
	}
	if f.count("/account/positions") != 0 || f.count("/account/bills-archive") != 2 {
		t.Errorf("closed scope must skip open positions and fetch SWAP+FUTURES bills: %v", f.hits)
	}
	closed, _ := closedOnly.ClosedPositions()
	if summarize(closed) != summarize(fullClosed) {
		t.Errorf("closed positions differ:\n%s\n%s", summarize(closed), summarize(fullClosed))
	}

	f.reset()
	openOnly, err := Load(client, f.URL, nil, scope.Open)
	if err != nil {
		t.Fatal(err)
	}
	if f.count("/account/bills-archive") != 0 || f.count("/account/balance") != 0 || f.count("/account/positions-history") != 0 {
		t.Errorf("open scope must only fetch positions, orders, fills and instruments: %v", f.hits)
	}
	open, _ := openOnly.OpenPositions()
	if len(open) != len(fullOpen) || len(open[0].Orders) != len(fullOpen[0].Orders) || open[0].Amount != fullOpen[0].Amount {
		t.Errorf("open positions differ:\n%v\n%v", open, fullOpen)
	}

	f.reset()
	fundingsOnly, err := Load(client, f.URL, &cutoff, scope.Fundings)
	if err != nil {
		t.Fatal(err)
	}
	if f.count("/account/bills-archive") != 2 || f.count("/account/balance") != 0 {
		t.Errorf("fundings scope must fetch bills only: %v", f.hits)
	}
	if fmt.Sprint(fundingsOnly.Fundings()) != fmt.Sprint(all.Fundings()) {
		t.Errorf("fundings differ: %v vs %v", fundingsOnly.Fundings(), all.Fundings())
	}

	f.reset()
	txOnly, err := Load(client, f.URL, &cutoff, scope.Transactions|scope.Balances)
	if err != nil {
		t.Fatal(err)
	}
	if f.count("/account/bills-archive") != 1 || f.count("/account/balance") != 1 {
		t.Errorf("transactions+balances scope: %v", f.hits)
	}
	if fmt.Sprint(txOnly.Transactions()) != fmt.Sprint(all.Transactions()) {
		t.Errorf("transactions differ")
	}
	if balances(txOnly.BalanceSnapshots()) != balances(all.BalanceSnapshots()) {
		t.Errorf("snapshots differ: %v vs %v", txOnly.BalanceSnapshots(), all.BalanceSnapshots())
	}
}

// balances lists the snapshot balances; the trailing "now" snapshot carries
// a fresh timestamp on every load.
func balances(snapshots []domain.UserBalanceSnapshot) string {
	var out []string
	for _, s := range snapshots {
		out = append(out, fmt.Sprint(s.Balance))
	}
	return strings.Join(out, ",")
}

// summarize renders the fields of every position that do not change between
// loads (ids are generated, MAE/MFE are pointers).
func summarize(positions []domain.Position) string {
	var out []string
	deref := func(v *float64) string {
		if v == nil {
			return "nil"
		}
		return fmt.Sprint(*v)
	}
	for _, p := range positions {
		line := fmt.Sprintf("%s %s amt=%v entry=%v exit=%v net=%v fee=%v funding=%v mae=%s mfe=%s init=%v created=%s orders=",
			p.Pair, p.Side, p.Amount, p.EntryPrice, p.ExitPrice, p.NetPnl, p.Commission, p.Funding, deref(p.MAE), deref(p.MFE), p.BalanceInit, p.CreatedAt)
		for _, o := range p.Orders {
			line += fmt.Sprintf("[%s %s %v@%v fee=%v]", o.ExchangeOrderID, o.Side, o.AmountFilled, o.AveragePrice, o.Trade.Commission)
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
