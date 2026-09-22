package executors

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

// Three pages newest first; sinceMs falls inside the second page, so the
// third must never be requested.
func TestFetchClosedPositionsStopsAtSince(t *testing.T) {
	now := time.Now().UnixMilli()
	day := int64(24 * 3600 * 1000)
	since := now - 5*day

	rows := func(from, to int) string {
		s := ""
		for i := from; i < to; i++ {
			if s != "" {
				s += ","
			}
			s += fmt.Sprintf(`{"instId":"P%d","instType":"SWAP","uTime":"%d","cTime":"%d"}`, i, now-int64(i)*day/50, now-int64(i)*day/50-day)
		}
		return s
	}
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		page := 0
		if after := r.URL.Query().Get("after"); after != "" {
			var last int64
			fmt.Sscan(after, &last)
			// rows are uTime = now - i*day/50: the row after uTime `last`.
			page = int((now-last)*50/day+1) / positionsPageLimit
		}
		body := rows(page*positionsPageLimit, (page+1)*positionsPageLimit)
		_, _ = w.Write([]byte(`{"code":"0","data":[` + body + `]}`))
	}))
	defer server.Close()

	got, err := FetchAllClosedPositionsByInstType(resty.New(), server.URL, "SWAP", since)
	if err != nil {
		t.Fatal(err)
	}
	// uTime = now - i*day/50 >= since  <=>  i <= 250: pages 1-2 fully,
	// page 3 stops at its 52nd row; no fourth page.
	if requests != 3 {
		t.Errorf("requests = %d, want 3", requests)
	}
	if len(got) != 251 {
		t.Errorf("rows = %d, want 251", len(got))
	}

	requests = 0
	got, err = FetchAllClosedPositionsByInstType(resty.New(), server.URL, "SWAP", now-2*day)
	if err != nil {
		t.Fatal(err)
	}
	// i <= 100 qualify: page 1 fully, page 2 stops at its second row.
	if requests != 2 {
		t.Errorf("requests = %d, want 2", requests)
	}
	if len(got) != positionsPageLimit+1 {
		t.Errorf("rows = %d, want %d", len(got), positionsPageLimit+1)
	}
}
