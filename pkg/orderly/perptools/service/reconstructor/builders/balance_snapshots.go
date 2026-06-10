package builders

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/models"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/helpers"
)

type syntheticBalanceEvent struct {
	At    time.Time
	Delta float64
}

func isCompletedTransfer(ev models.OrderlyAssetHistory) bool {
	return strings.EqualFold(ev.TransStatus, "COMPLETED")
}

func BuildSyntheticBalanceSnapshotsFromStableTransfersAndClosedPositions(
	assetHistory []models.OrderlyAssetHistory,
	positions []domain.Position,
) []domain.UserBalanceSnapshot {
	stableHistory := make([]models.OrderlyAssetHistory, 0, len(assetHistory))
	for _, ev := range assetHistory {
		if isStableToken(ev.Token) {
			stableHistory = append(stableHistory, ev)
		}
	}
	snapshots, _ := BuildSyntheticBalanceSnapshotsFromTransfersAndClosedPositions(stableHistory, positions, nil)
	return snapshots
}

func BuildSyntheticBalanceSnapshotsFromTransfersAndClosedPositions(
	assetHistory []models.OrderlyAssetHistory,
	positions []domain.Position,
	markPrices map[string]float64,
) ([]domain.UserBalanceSnapshot, error) {
	events := make([]syntheticBalanceEvent, 0, len(assetHistory)+len(positions))

	var firstDepositAt *time.Time
	for _, ev := range assetHistory {
		if !isCompletedTransfer(ev) {
			continue
		}

		price := tokenUSDCPrice(ev.Token, markPrices)
		if price <= 0 {
			return nil, fmt.Errorf("missing USDC mark price for token %q", ev.Token)
		}

		side := strings.ToUpper(strings.TrimSpace(ev.Side))
		at := time.UnixMilli(ev.CreatedTime).UTC()

		switch side {
		case "DEPOSIT":
			delta := math.Abs(ev.Amount) * price
			events = append(events, syntheticBalanceEvent{At: at, Delta: delta})
			if firstDepositAt == nil || at.Before(*firstDepositAt) {
				tmp := at
				firstDepositAt = &tmp
			}
		case "WITHDRAW", "WITHDRAWAL":
			delta := -math.Abs(ev.Amount) * price
			events = append(events, syntheticBalanceEvent{At: at, Delta: delta})
		}
	}

	if firstDepositAt == nil {
		return nil, nil
	}

	start := time.Date(
		firstDepositAt.Year(),
		firstDepositAt.Month(),
		firstDepositAt.Day(),
		0, 0, 0, 0, time.UTC,
	)

	for _, pos := range positions {
		if !pos.Closed || pos.ClosedAt == nil {
			continue
		}
		if pos.ClosedAt.UTC().Before(start) {
			continue
		}
		events = append(events, syntheticBalanceEvent{
			At:    pos.ClosedAt.UTC(),
			Delta: pos.NetPnl,
		})
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].At.Before(events[j].At)
	})

	snapshots := make([]domain.UserBalanceSnapshot, 0, len(events)+1)
	snapshots = append(snapshots, domain.UserBalanceSnapshot{
		CreatedAt: start,
		Balance:   0,
	})

	balance := 0.0
	for _, ev := range events {
		if ev.At.Before(start) {
			continue
		}
		balance = helpers.Round8(balance + ev.Delta)
		snapshots = append(snapshots, domain.UserBalanceSnapshot{
			CreatedAt: ev.At,
			Balance:   balance,
		})
	}

	return snapshots, nil
}

func isStableToken(token string) bool {
	return strings.Contains(strings.ToUpper(strings.TrimSpace(token)), "USD")
}

func tokenUSDCPrice(token string, markPrices map[string]float64) float64 {
	token = canonicalAssetToken(token)
	if token == "" {
		return 0
	}
	if isStableToken(token) {
		return 1
	}
	return markPrices[token]
}

func canonicalAssetToken(token string) string {
	token = strings.ToUpper(strings.TrimSpace(token))
	token = strings.ReplaceAll(token, "-", "_")
	token = strings.ReplaceAll(token, "/", "_")
	token = strings.ReplaceAll(token, " ", "_")
	token = strings.Trim(token, "_")
	token = strings.TrimPrefix(token, "PERP_")
	for _, suffix := range []string{"_PERP", "_USDC", "_USDT", "_USD"} {
		token = strings.TrimSuffix(token, suffix)
	}
	return token
}
