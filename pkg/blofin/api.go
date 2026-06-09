package blofin

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	blofinclient "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func GetAuthStatus(
	client *resty.Client,
	creds blofinclient.Credentials,
) string {
	client = authClient(client, creds)
	if err := executors.FetchAuthStatus(client); err != nil {
		return "error"
	}
	return "ok"
}

func GetBuiltPositions(
	client *resty.Client,
	creds blofinclient.Credentials,
	days int,
) ([]domain.Position, error) {
	client = authClient(client, creds)

	allPositions, err := reconstructor.ReconstructClosedPositions(client)
	if err != nil {
		return nil, err
	}

	positions := make([]domain.Position, len(allPositions))
	copy(positions, allPositions)
	positions = helpers.FilterPositionsByDays(positions, days)

	currentBalance, err := fetchFuturesEquity(client)
	if err == nil {
		bills, billsErr := executors.FetchAllAssetBills(client)
		if billsErr == nil {
			snapshots := builders.BuildSyntheticBalanceSnapshots(currentBalance, bills, allPositions)
			builders.AttachBalanceInit(&positions, snapshots)
		}
	}

	return positions, nil
}

func GetClosedPositionByExactMatch(
	client *resty.Client,
	creds blofinclient.Credentials,
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
	creds blofinclient.Credentials,
) ([]domain.OpenPosition, error) {
	client = authClient(client, creds)

	raw, err := executors.FetchOpenPositions(client)
	if err != nil {
		return nil, err
	}

	instruments, err := executors.FetchAllInstruments(client)
	if err != nil {
		return nil, err
	}
	contractValues := helpers.ContractValueByInstID(instruments)

	positions := make([]domain.OpenPosition, 0, len(raw))
	filteredRaw := make([]models.OpenPosition, 0, len(raw))
	for _, r := range raw {
		if helpers.MustFloat(r.Positions) == 0 {
			continue
		}
		filteredRaw = append(filteredRaw, r)
		positions = append(positions, builders.BuildOpenPosition(r, contractValues[r.InstID]))
	}
	enrichOpenPositionOrders(client, filteredRaw, positions, contractValues)
	return positions, nil
}

func GetBalanceSnapshots(
	client *resty.Client,
	creds blofinclient.Credentials,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	client = authClient(client, creds)

	currentBalance, err := fetchFuturesEquity(client)
	if err != nil {
		return nil, err
	}

	positions, err := reconstructor.ReconstructClosedPositions(client)
	if err != nil {
		return nil, err
	}

	bills, err := executors.FetchAllAssetBills(client)
	if err != nil {
		return nil, err
	}

	snapshots := builders.BuildSyntheticBalanceSnapshots(currentBalance, bills, positions)
	snapshots = helpers.FilterSnapshotsByDays(snapshots, days)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	return snapshots, nil
}

func GetCurrentBalance(
	client *resty.Client,
	creds blofinclient.Credentials,
) (*float64, error) {
	client = authClient(client, creds)

	val, err := fetchFuturesEquity(client)
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func GetFundings(
	client *resty.Client,
	creds blofinclient.Credentials,
	days int,
) ([]domain.UserFunding, error) {
	_ = authClient(client, creds)
	_ = days
	return []domain.UserFunding{}, nil
}

func GetCandles(
	client *resty.Client,
	instID string,
	interval string,
	startTime time.Time,
	endTime time.Time,
) ([]models.Candle, error) {
	if client == nil {
		client = blofinclient.NewBaseClient()
	}
	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be >= startTime")
	}
	return executors.FetchCandles(
		client,
		instID,
		interval,
		startTime.UnixMilli(),
		endTime.UnixMilli(),
	)
}

func enrichOpenPositionOrders(
	client *resty.Client,
	raw []models.OpenPosition,
	positions []domain.OpenPosition,
	contractValues map[string]float64,
) {
	if len(raw) == 0 || len(positions) == 0 {
		return
	}

	startMs := int64(0)
	for _, r := range raw {
		t := helpers.MustInt64(r.CreateTime)
		if t > 0 && (startMs == 0 || t < startMs) {
			startMs = t
		}
	}
	if startMs == 0 {
		return
	}
	startMs -= 10 * 60 * 1000

	orders, err := executors.FetchAllOrdersHistory(client, startMs)
	if err != nil {
		return
	}

	for i := range positions {
		if i >= len(raw) {
			return
		}
		r := raw[i]
		matched := make([]models.Order, 0)
		for _, ord := range orders {
			if ord.InstID != r.InstID {
				continue
			}
			if r.PositionID != "" && ord.PositionID != "" && r.PositionID != ord.PositionID {
				continue
			}
			if !samePositionSide(r.PositionSide, ord.PositionSide) {
				continue
			}
			if helpers.OrderTimeMs(ord) < helpers.MustInt64(r.CreateTime) {
				continue
			}
			matched = append(matched, ord)
		}
		positions[i].Orders = builders.BuildOrders(matched, positions[i].ID, contractValues[r.InstID])
	}
}

func fetchFuturesEquity(client *resty.Client) (float64, error) {
	balance, err := executors.FetchBalance(client)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(balance.TotalEquity) != "" {
		return helpers.Round8(helpers.MustFloat(balance.TotalEquity)), nil
	}

	var total float64
	var found bool
	for _, detail := range balance.Details {
		if !helpers.IsStableCurrency(detail.Currency) {
			continue
		}
		equity := helpers.MustFloat(detail.EquityUSD)
		if equity == 0 {
			equity = helpers.MustFloat(detail.Equity)
		}
		total += equity
		found = true
	}
	if !found {
		return 0, fmt.Errorf("blofin: futures equity not found")
	}
	return helpers.Round8(total), nil
}

func authClient(client *resty.Client, creds blofinclient.Credentials) *resty.Client {
	if client == nil {
		return blofinclient.NewClient(creds)
	}
	blofinclient.AttachAuth(client, creds)
	return client
}

func samePositionSide(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	return a == "" || b == "" || a == "net" || b == "net" || a == b
}
