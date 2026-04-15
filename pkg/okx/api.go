package okx

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	okxclient "github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/workers"
)

const defaultCandleWorkers = 4

func GetAuthStatus(apiKey, secret, passphrase string) (string, okxclient.Region) {
	region, err := okxclient.CheckAccount(apiKey, secret, passphrase)
	if err != nil {
		return "error", ""
	}

	return "ok", region
}

func GetBuiltPositions(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
	days int,
) ([]domain.Position, error) {
	okxclient.AttachAuth(client, creds)

	closedPositions, err := executors.FetchAllClosedPositions(client, baseURL)
	if err != nil {
		return nil, err
	}
	if len(closedPositions) == 0 {
		return []domain.Position{}, nil
	}

	oldestMs := helpers.MustInt64(closedPositions[0].CTime)
	for _, cp := range closedPositions[1:] {
		if t := helpers.MustInt64(cp.CTime); t < oldestMs {
			oldestMs = t
		}
	}

	oldestMs -= 10 * 60 * 1000

	allOrders, err := executors.FetchAllSwapAndFuturesOrders(client, baseURL, oldestMs)
	if err != nil {
		return nil, err
	}
	ordersByInst := helpers.GroupOrdersByInst(allOrders)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, baseURL, candleRequests, defaultCandleWorkers)

	type pendingCandle struct {
		idx     int
		replyCh chan helpers.CandleResponse
	}

	pending := make([]pendingCandle, 0, len(closedPositions))
	positions := make([]domain.Position, len(closedPositions))

	for i, cp := range closedPositions {
		posOrders := helpers.MatchOrdersToPosition(cp, ordersByInst)
		pos, err := helpers.BuildPosition(cp, posOrders)
		if err != nil {
			continue
		}
		positions[i] = pos

		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			InstId:  cp.InstId,
			Bar:     "1m",
			StartMs: helpers.MustInt64(cp.CTime),
			EndMs:   helpers.MustInt64(cp.UTime),
			ReplyCh: replyCh,
		}
		pending = append(pending, pendingCandle{idx: i, replyCh: replyCh})
	}
	close(candleRequests)

	for _, p := range pending {
		resp := <-p.replyCh
		if resp.Err == nil {
			high, low := helpers.GetHighLow(resp.Candles)
			helpers.ApplyMAEMFE(&positions[p.idx], high, low)
		}
	}

	filtered := make([]domain.Position, 0, len(positions))
	for _, pos := range positions {
		if pos.ID != uuid.Nil {
			filtered = append(filtered, pos)
		}
	}
	positions = filtered

	sort.Slice(positions, func(i, j int) bool {
		return positions[i].ClosedAt.Before(*positions[j].ClosedAt)
	})

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

	balance, err := executors.FetchBalance(client, baseURL)
	if err == nil {
		currentBal := helpers.MustFloat(balance.TotalEq)
		bills, billsErr := executors.FetchAllSwapAndFuturesBills(client, baseURL, oldestMs, "")
		if billsErr == nil && len(bills) > 0 {
			snapshots := builders.BuildBalanceSnapshotsFromBills(currentBal, bills)
			helpers.AttachBalanceInit(&positions, snapshots)
		}
	}

	return positions, nil
}

func GetClosedPositionByExactMatch(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	positions, err := GetBuiltPositions(client, creds, baseURL, 0)
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
	creds okxclient.Credentials,
	baseURL string,
) ([]domain.OpenPosition, error) {
	okxclient.AttachAuth(client, creds)

	raw, err := executors.FetchOpenPositions(client, baseURL)
	if err != nil {
		return nil, err
	}

	positions := make([]domain.OpenPosition, 0, len(raw))
	for _, r := range raw {
		positions = append(positions, builders.BuildOpenPosition(r))
	}
	return positions, nil
}

func GetBalanceSnapshots(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	okxclient.AttachAuth(client, creds)

	balance, err := executors.FetchBalance(client, baseURL)
	if err != nil {
		return nil, err
	}
	currentBal := helpers.MustFloat(balance.TotalEq)

	startMs := int64(0)
	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		startMs = cutoff.UnixMilli()
	}

	bills, err := executors.FetchAllSwapAndFuturesBills(client, baseURL, startMs, "")
	if err != nil {
		return nil, err
	}
	snapshots := builders.BuildBalanceSnapshotsFromBills(currentBal, bills)
	if cutoff != nil {
		filtered := snapshots[:0]
		for _, s := range snapshots {
			if !s.CreatedAt.Before(*cutoff) {
				filtered = append(filtered, s)
			}
		}
		snapshots = filtered
	}

	return snapshots, nil
}

func GetCurrentBalance(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
) (*float64, error) {
	okxclient.AttachAuth(client, creds)

	balance, err := executors.FetchBalance(client, baseURL)
	if err != nil {
		return nil, err
	}

	val := helpers.MustFloat(balance.TotalEq)
	return &val, nil
}

func GetFundings(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
	days int,
) ([]domain.UserFunding, error) {
	okxclient.AttachAuth(client, creds)

	startMs := int64(0)
	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		startMs = cutoff.UnixMilli()
	}

	bills, err := executors.FetchAllSwapAndFuturesBills(client, baseURL, startMs, "8")
	if err != nil {
		return nil, err
	}

	fundings := make([]domain.UserFunding, 0, len(bills))
	for _, b := range bills {
		amount := helpers.MustFloat(b.BalChg)
		if amount == 0 {
			continue
		}
		fundings = append(fundings, domain.UserFunding{
			Pair:      helpers.NormalizePair(b.InstId),
			Amount:    helpers.Round8(amount),
			CreatedAt: helpers.TimeFromMs(b.Ts),
		})
	}

	return fundings, nil
}

func GetCandles(
	client *resty.Client,
	baseURL string,
	instId string,
	bar string,
	startTime time.Time,
	endTime time.Time,
) ([]models.Candle, error) {
	if client == nil {
		client = okxclient.NewPublicClient()
	}

	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be >= startTime")
	}

	return executors.FetchCandles(
		client, baseURL, instId, bar,
		startTime.UnixMilli(), endTime.UnixMilli(),
	)
}
