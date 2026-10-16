package executors

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

const (
	closedSpan      = 30 * 24 * time.Hour
	maxEmptyWindows = 12
	maxHistory      = 10 * 365 * 24 * time.Hour
)

type rangeBody struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func FetchClosedPositions(ctx context.Context, c *connector.Client, now, floor time.Time) ([]models.ClosedPosition, error) {
	unbounded := floor.IsZero()
	if unbounded {
		floor = now.Add(-maxHistory)
	}
	seen := make(map[string]struct{})
	var out []models.ClosedPosition
	empty := 0
	for _, span := range window.Backward(now.UnixMilli(), floor.UnixMilli()-1, closedSpan.Milliseconds()) {
		body := rangeBody{From: models.FormatTime(time.UnixMilli(span.StartMs)), To: models.FormatTime(time.UnixMilli(span.EndMs))}
		var raw json.RawMessage
		if err := c.Do(ctx, http.MethodPost, "closed-positions", nil, body, &raw); err != nil {
			return nil, err
		}
		page, err := decodeList[models.ClosedPosition]("closed-positions", raw, "operations", "positions")
		if err != nil {
			return nil, err
		}
		for _, p := range page {
			key := p.Key()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, p)
		}
		if len(page) == 0 {
			empty++
		} else {
			empty = 0
		}
		if unbounded && empty >= maxEmptyWindows {
			break
		}
	}
	return out, nil
}
