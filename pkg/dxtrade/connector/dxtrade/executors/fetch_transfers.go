package executors

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"sort"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
)

func FetchTransfers(ctx context.Context, c *connector.Client, from *time.Time) ([]models.CashTransfer, error) {
	path, err := c.AccountPath(ctx, "/transfers")
	if err != nil {
		return nil, err
	}
	base := url.Values{}
	if from != nil {
		base.Set("time-from", models.FormatTime(*from))
	}
	seenTx := make(map[string]bool)
	seenTransfer := make(map[string]bool)
	var out []models.CashTransfer
	cursor := ""
	for page := 0; page < maxPages; page++ {
		query := maps.Clone(base)
		if cursor != "" {
			query.Set("time-to", cursor)
		}
		var list models.CashTransferList
		if err := c.Do(ctx, "transfers", http.MethodGet, path, query, nil, &list); err != nil {
			return nil, err
		}
		for _, t := range list.CashTransfers {
			if kept, ok := dedupeTransfer(t, seenTx, seenTransfer); ok {
				out = append(out, kept)
			}
		}
		next := list.NextPageTransactionTime.Raw
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TransactionTime.Before(out[j].TransactionTime.Time) })
	return out, nil
}

func dedupeTransfer(t models.CashTransfer, seenTx, seenTransfer map[string]bool) (models.CashTransfer, bool) {
	if len(t.CashTransactions) == 0 {
		key := t.TransferCode
		if key == "" {
			key = fmt.Sprintf("%s|%v", t.TransactionTime.Raw, t.Version)
		}
		if seenTransfer[key] {
			return t, false
		}
		seenTransfer[key] = true
		return t, true
	}
	kept := t.CashTransactions[:0:0]
	for _, tx := range t.CashTransactions {
		key := tx.Key()
		if seenTx[key] {
			continue
		}
		seenTx[key] = true
		kept = append(kept, tx)
	}
	if len(kept) == 0 {
		return t, false
	}
	t.CashTransactions = kept
	return t, true
}
