package mexc

import (
	"fmt"
	"sort"
	"strings"
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

	currentEquity, err := fetchStableEquity(client)
	if err == nil {
		transfers, trErr := executors.FetchAllTransferRecords(client)
		if trErr == nil && len(transfers) > 0 {
			snapshots := builders.BuildSyntheticBalanceSnapshots(currentEquity, transfers, positions)
			builders.AttachBalanceInit(&positions, snapshots)
		}
	}

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

	currentEquity, err := fetchStableEquity(client)
	if err != nil {
		return nil, err
	}

	transfers, err := executors.FetchAllTransferRecords(client)
	if err != nil {
		return nil, err
	}

	snapshots := builders.BuildSyntheticBalanceSnapshots(currentEquity, transfers, positions)

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

	val, err := fetchStableEquity(client)
	if err != nil {
		return nil, err
	}

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

func fetchStableEquity(client *resty.Client) (float64, error) {
	assets, err := executors.FetchAssets(client)
	if err == nil {
		var total float64
		var found bool
		for _, asset := range assets {
			if isStableCurrency(asset.Currency) {
				total += asset.Equity
				found = true
			}
		}
		if found {
			return helpers.Round8(total), nil
		}
	}

	asset, fallbackErr := executors.FetchUSDTAsset(client)
	if fallbackErr != nil {
		if err != nil {
			return 0, err
		}
		return 0, fallbackErr
	}
	return helpers.Round8(asset.Equity), nil
}

func isStableCurrency(currency string) bool {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "USDT", "USDC":
		return true
	default:
		return false
	}
}
