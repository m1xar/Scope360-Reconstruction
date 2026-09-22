package executors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

// Three full pages newest first (updateTime 300..1). With sinceMs inside
// page 1, page 2 is entirely older and paging stops there; page 2 is still
// returned whole (filtering happens later).
func TestFetchAllHistoryPositionsStopsAtSince(t *testing.T) {
	const total = 3 * positionsPageSize
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		page, _ := strconv.Atoi(r.URL.Query().Get("page_num"))
		var rows []models.HistoryPosition
		for i := (page - 1) * positionsPageSize; i < page*positionsPageSize && i < total; i++ {
			rows = append(rows, models.HistoryPosition{PositionId: int64(i), UpdateTime: int64(total - i)})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "code": 0, "data": rows})
	}))
	defer server.Close()

	client := resty.New().SetTransport(redirectTo(server.URL))
	got, err := FetchAllHistoryPositions(client, 250)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Errorf("requests = %d, want 2", requests)
	}
	if len(got) != 2*positionsPageSize {
		t.Errorf("rows = %d, want %d", len(got), 2*positionsPageSize)
	}

	requests = 0
	if _, err := FetchAllHistoryPositions(client, 0); err != nil {
		t.Fatal(err)
	}
	if requests != 4 {
		t.Errorf("sinceMs=0: requests = %d, want 4 (three full pages, then an empty one)", requests)
	}
}

type redirectTo string

func (r redirectTo) RoundTrip(req *http.Request) (*http.Response, error) {
	target, _ := url.Parse(string(r))
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	return http.DefaultTransport.RoundTrip(req)
}
