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
	FinanceSpan  = 365 * 24 * time.Hour
	financeLimit = 1000
)

type financeBody struct {
	Types []int  `json:"types"`
	From  string `json:"from"`
	To    string `json:"to"`
	Limit int    `json:"limit"`
}

func FetchFinance(ctx context.Context, c *connector.Client, now, floor time.Time) (ops []models.FinanceOperation, available bool, err error) {
	if floor.IsZero() {
		floor = now.Add(-maxHistory)
	}
	seen := make(map[string]struct{})
	for _, span := range window.Backward(now.UnixMilli(), floor.UnixMilli()-1, FinanceSpan.Milliseconds()) {
		body := financeBody{Types: models.FinanceTypes, From: models.FormatTime(time.UnixMilli(span.StartMs)), To: models.FormatTime(time.UnixMilli(span.EndMs)), Limit: financeLimit}
		var raw json.RawMessage
		if err := c.Do(ctx, http.MethodPost, "finance", nil, body, &raw); err != nil {
			if connector.IsStatus(err, http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed) {
				return nil, false, nil
			}
			return nil, false, err
		}
		page, err := decodeList[models.FinanceOperation]("finance", raw, "operations", "ledgers")
		if err != nil {
			return nil, false, err
		}
		for _, op := range page {
			key := op.ID.String()
			if key == "" {
				key = op.Kind() + op.At().Format(time.RFC3339Nano)
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			ops = append(ops, op)
		}
	}
	return ops, true, nil
}
