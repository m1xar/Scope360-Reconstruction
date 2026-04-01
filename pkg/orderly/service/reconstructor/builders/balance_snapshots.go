package builders

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/service/reconstructor/helpers"
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
	events := make([]syntheticBalanceEvent, 0, len(assetHistory)+len(positions))

	var firstStableDepositAt *time.Time
	for _, ev := range assetHistory {
		if !isCompletedTransfer(ev) || !isStableToken(ev.Token) {
			continue
		}

		side := strings.ToUpper(strings.TrimSpace(ev.Side))
		at := time.UnixMilli(ev.CreatedTime).UTC()

		switch side {
		case "DEPOSIT":
			delta := math.Abs(ev.Amount)
			events = append(events, syntheticBalanceEvent{At: at, Delta: delta})
			if firstStableDepositAt == nil || at.Before(*firstStableDepositAt) {
				tmp := at
				firstStableDepositAt = &tmp
			}
		case "WITHDRAW", "WITHDRAWAL":
			delta := -(math.Abs(ev.Amount) + math.Abs(ev.Fee))
			events = append(events, syntheticBalanceEvent{At: at, Delta: delta})
		}
	}

	if firstStableDepositAt == nil {
		return nil
	}

	start := time.Date(
		firstStableDepositAt.Year(),
		firstStableDepositAt.Month(),
		firstStableDepositAt.Day(),
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

	return snapshots
}

func isStableToken(token string) bool {
	return strings.Contains(strings.ToUpper(strings.TrimSpace(token)), "USD")
}
