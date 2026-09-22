package executors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/connector/binance/models"
)

func TestFetchUserTradesWindowPagesByStartTime(t *testing.T) {
	// 1500 trades at t = 1000+i; two pages of 1000, second page starts at the
	// max time of the first (inclusive, so one duplicate to dedupe).
	all := make([]models.Trade, 1500)
	for i := range all {
		all[i] = models.Trade{Symbol: "BTCUSDT", ID: int64(i + 1), Time: int64(1000 + i), Qty: "1"}
	}
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("fromId") != "" {
			t.Fatal("fromId must not be sent with a time range")
		}
		start, _ := strconv.ParseInt(r.URL.Query().Get("startTime"), 10, 64)
		end, _ := strconv.ParseInt(r.URL.Query().Get("endTime"), 10, 64)
		var page []models.Trade
		for _, tr := range all {
			if tr.Time >= start && tr.Time <= end && len(page) < tradesPageLimit {
				page = append(page, tr)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer server.Close()

	client := resty.New().SetTransport(redirectTo(server.URL))
	got, err := FetchUserTradesWindow(client, "BTCUSDT", 1000, 2499)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1500 {
		t.Fatalf("got %d trades, want 1500", len(got))
	}
	if requests != 2 {
		t.Errorf("requests = %d, want 2", requests)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Time < got[i-1].Time {
			t.Fatal("result not sorted")
		}
	}
}

// redirectTo sends every request to the test server: DoGet builds URLs from
// the constant BaseURL.
type redirectTo string

func (r redirectTo) RoundTrip(req *http.Request) (*http.Response, error) {
	target, _ := url.Parse(string(r))
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	return http.DefaultTransport.RoundTrip(req)
}
