package mexc

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	mexcclient "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
)

func GetAuthStatus(client *resty.Client, creds mexcclient.Credentials) string {
	mexcclient.AttachAuth(client, creds)

	_, err := executors.FetchUSDTAsset(client)
	if err != nil {
		return "error"
	}

	return "ok"
}

func GetBuiltPositions(
	client *resty.Client,
	creds mexcclient.Credentials,
	days int,
) ([]domain.Position, error) {
	mexcclient.AttachAuth(client, creds)

	positions, err := reconstructor.ReconstructClosedPositions(client)
	if err != nil {
		return nil, err
	}

	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		trimmed := positions[:0]
		for _, pos := range positions {
			if pos.ClosedAt != nil && !pos.ClosedAt.Before(*cutoff) {
				trimmed = append(trimmed, pos)
			}
		}
		positions = trimmed
	}

	builders.AttachNotionalBalanceInit(&positions)

	return positions, nil
}

func GetClosedPositionByExactMatch(
	client *resty.Client,
	creds mexcclient.Credentials,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	positions, err := GetBuiltPositions(client, creds, 0)
	if err != nil {
		return nil, err
	}

	for i := range positions {
		pos := &positions[i]
		if pos.Pair == pair && pos.Side == side && pos.CreatedAt.Equal(openedAt) {
			return pos, nil
		}
	}
	return nil, nil
}

func GetOpenPositions(
	client *resty.Client,
	creds mexcclient.Credentials,
) ([]domain.OpenPosition, error) {
	mexcclient.AttachAuth(client, creds)

	raw, err := executors.FetchOpenPositions(client)
	if err != nil {
		return nil, err
	}

	positions := make([]domain.OpenPosition, 0, len(raw))
	for _, r := range raw {
		if r.HoldVol <= 0 {
			continue
		}
		positions = append(positions, builders.BuildOpenPosition(r))
	}
	return positions, nil
}

func GetBalanceSnapshots(
	client *resty.Client,
	creds mexcclient.Credentials,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	mexcclient.AttachAuth(client, creds)

	positions, err := reconstructor.ReconstructClosedPositions(client)
	if err != nil {
		return nil, err
	}

	snapshots := builders.BuildZeroBalanceSnapshots(positions)

	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		filtered := snapshots[:0]
		for _, s := range snapshots {
			if !s.CreatedAt.Before(*cutoff) {
				filtered = append(filtered, s)
			}
		}
		snapshots = filtered
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	return snapshots, nil
}

func GetCurrentBalance(
	client *resty.Client,
	creds mexcclient.Credentials,
) (*float64, error) {
	mexcclient.AttachAuth(client, creds)

	asset, err := executors.FetchUSDTAsset(client)
	if err != nil {
		return nil, err
	}

	val := helpers.Round8(asset.Equity)
	return &val, nil
}

func GetFundings(
	client *resty.Client,
	creds mexcclient.Credentials,
	days int,
) ([]domain.UserFunding, error) {
	mexcclient.AttachAuth(client, creds)

	records, err := executors.FetchAllFundingRecords(client)
	if err != nil {
		return nil, err
	}

	fundings := builders.BuildUserFundings(records)

	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		filtered := fundings[:0]
		for _, f := range fundings {
			if !f.CreatedAt.Before(*cutoff) {
				filtered = append(filtered, f)
			}
		}
		fundings = filtered
	}

	return fundings, nil
}

func GetCandles(
	client *resty.Client,
	symbol string,
	interval string,
	startTime time.Time,
	endTime time.Time,
) ([]models.Candle, error) {
	if client == nil {
		client = mexcclient.NewPublicClient()
	}

	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be >= startTime")
	}

	return executors.FetchCandles(
		client, symbol, interval,
		startTime.UnixMilli(), endTime.UnixMilli(),
	)
}
