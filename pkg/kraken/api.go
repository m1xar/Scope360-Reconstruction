package kraken

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	krakenclient "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/workers"
)

const defaultCandleWorkers = 4

func authClient(client *resty.Client, creds krakenclient.Credentials) *resty.Client {
	if client == nil {
		return krakenclient.NewClient(creds)
	}
	krakenclient.AttachAuth(client, creds)
	return client
}

func GetAuthStatus(client *resty.Client, creds krakenclient.Credentials) string {
	client = authClient(client, creds)
	if _, err := executors.CheckAPIKey(client); err != nil {
		return "error"
	}
	return "ok"
}

func GetBuiltPositions(
	client *resty.Client,
	creds krakenclient.Credentials,
	days int,
) ([]domain.Position, error) {
	client = authClient(client, creds)

	fills, err := executors.FetchAllFills(client, days)
	if err != nil {
		return nil, err
	}
	if len(fills) == 0 {
		return []domain.Position{}, nil
	}

	positionEvents, err := executors.FetchAllPositionEvents(client, days)
	if err != nil {
		return nil, err
	}

	pairBySymbol := buildPairMap(client, symbolsFromFillsAndEvents(fills, positionEvents))
	positions, err := builders.BuildClosedPositions(fills, positionEvents, pairBySymbol)
	if err != nil {
		return nil, err
	}

	enrichMAEMFE(client, &positions, rawSymbolByPair(symbolsFromFillsAndEvents(fills, positionEvents), pairBySymbol))

	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		filtered := positions[:0]
		for _, pos := range positions {
			if pos.ClosedAt != nil && !pos.ClosedAt.Before(*cutoff) {
				filtered = append(filtered, pos)
			}
		}
		positions = filtered
	}

	if logs, err := executors.FetchAllAccountLog(client, days); err == nil {
		snapshots := builders.BuildBalanceSnapshots(logs)
		helpers.AttachBalanceInit(&positions, snapshots)
	}

	return positions, nil
}

func GetClosedPositionByExactMatch(
	client *resty.Client,
	creds krakenclient.Credentials,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	positions, err := GetBuiltPositions(client, creds, 0)
	if err != nil {
		return nil, err
	}

	pair = helpers.NormalizePairText(pair)
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
	creds krakenclient.Credentials,
) ([]domain.OpenPosition, error) {
	client = authClient(client, creds)

	rawPositions, err := executors.FetchOpenPositions(client)
	if err != nil {
		return nil, err
	}
	if len(rawPositions) == 0 {
		return []domain.OpenPosition{}, nil
	}

	tickers, err := executors.FetchTickers(client)
	if err != nil {
		return nil, err
	}
	tickerBySymbol := make(map[string]models.Ticker, len(tickers))
	for _, ticker := range tickers {
		tickerBySymbol[strings.ToUpper(ticker.Symbol)] = ticker
	}

	out := make([]domain.OpenPosition, 0, len(rawPositions))
	for _, pos := range rawPositions {
		if pos.Size.Float64() <= 0 {
			continue
		}
		ticker := tickerBySymbol[strings.ToUpper(pos.Symbol)]
		out = append(out, builders.BuildOpenPosition(pos, ticker))
	}
	return out, nil
}

func GetBalanceSnapshots(
	client *resty.Client,
	creds krakenclient.Credentials,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	client = authClient(client, creds)

	logs, err := executors.FetchAllAccountLog(client, days)
	if err != nil {
		return nil, err
	}
	snapshots := builders.BuildBalanceSnapshots(logs)

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
	creds krakenclient.Credentials,
) (*float64, error) {
	client = authClient(client, creds)

	accounts, err := executors.FetchAccounts(client)
	if err == nil {
		if val, ok := currentBalanceFromAccounts(accounts); ok {
			return &val, nil
		}
	}

	snapshots, snapErr := GetBalanceSnapshots(client, creds, 0)
	if snapErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, snapErr
	}
	if len(snapshots) == 0 {
		return nil, nil
	}
	val := snapshots[len(snapshots)-1].Balance
	return &val, nil
}

func GetFundings(
	client *resty.Client,
	creds krakenclient.Credentials,
	days int,
) ([]domain.UserFunding, error) {
	client = authClient(client, creds)

	logs, err := executors.FetchAllAccountLog(client, days)
	if err != nil {
		return nil, err
	}
	pairBySymbol := buildPairMap(client, symbolsFromAccountLogs(logs))
	fundings := builders.BuildFundings(logs, pairBySymbol)

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
	tickType string,
	symbol string,
	interval string,
	startTime time.Time,
	endTime time.Time,
) ([]models.Candle, error) {
	if client == nil {
		client = krakenclient.NewPublicClient()
	}

	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be >= startTime")
	}
	if tickType == "" {
		tickType = "trade"
	}

	return executors.FetchCandles(
		client,
		tickType,
		symbol,
		interval,
		startTime.UnixMilli(),
		endTime.UnixMilli(),
	)
}

func enrichMAEMFE(client *resty.Client, positions *[]domain.Position, symbolByPair map[string]string) {
	if positions == nil || len(*positions) == 0 {
		return
	}

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, candleRequests, defaultCandleWorkers)

	type pendingCandle struct {
		idx     int
		replyCh chan helpers.CandleResponse
	}
	pending := make([]pendingCandle, 0, len(*positions))

	for i := range *positions {
		pos := &(*positions)[i]
		if pos.ClosedAt == nil {
			continue
		}

		replyCh := make(chan helpers.CandleResponse, 1)
		symbol := symbolByPair[pos.Pair]
		if symbol == "" {
			symbol = symbolFromPair(pos.Pair)
		}

		candleRequests <- helpers.CandleRequest{
			TickType: "trade",
			Symbol:   symbol,
			Interval: "1m",
			StartMs:  pos.CreatedAt.UnixMilli(),
			EndMs:    pos.ClosedAt.UnixMilli(),
			ReplyCh:  replyCh,
		}
		pending = append(pending, pendingCandle{idx: i, replyCh: replyCh})
	}
	close(candleRequests)

	for _, p := range pending {
		resp := <-p.replyCh
		if resp.Err != nil {
			continue
		}
		high, low := helpers.GetHighLow(resp.Candles)
		builders.ApplyMAEMFE(&(*positions)[p.idx], high, low)
	}
}

func buildPairMap(client *resty.Client, symbols []string) map[string]string {
	out := make(map[string]string, len(symbols))
	for _, symbol := range symbols {
		sym := strings.ToUpper(strings.TrimSpace(symbol))
		if sym == "" {
			continue
		}
		if _, ok := out[sym]; ok {
			continue
		}
		ticker, err := executors.FetchTicker(client, sym)
		if err == nil && ticker.Pair != "" {
			out[sym] = helpers.NormalizePairText(ticker.Pair)
			continue
		}
		out[sym] = helpers.NormalizePairFallback(sym)
	}
	return out
}

func rawSymbolByPair(symbols []string, pairBySymbol map[string]string) map[string]string {
	out := make(map[string]string, len(symbols))
	for _, symbol := range symbols {
		sym := strings.ToUpper(strings.TrimSpace(symbol))
		if sym == "" {
			continue
		}
		pair := helpers.NormalizePair(sym, pairBySymbol)
		if _, ok := out[pair]; !ok {
			out[pair] = sym
		}
	}
	return out
}

func symbolsFromFillsAndEvents(fills []models.Fill, events []models.PositionEventElement) []string {
	set := make(map[string]struct{})
	for _, fill := range fills {
		set[strings.ToUpper(fill.Symbol)] = struct{}{}
	}
	for _, ev := range events {
		if ev.Event.PositionUpdate.Tradeable != "" {
			set[strings.ToUpper(ev.Event.PositionUpdate.Tradeable)] = struct{}{}
		}
	}
	return keys(set)
}

func symbolsFromAccountLogs(logs []models.AccountLog) []string {
	set := make(map[string]struct{})
	for _, row := range logs {
		if row.Contract != "" {
			set[strings.ToUpper(row.Contract)] = struct{}{}
		}
	}
	return keys(set)
}

func keys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func currentBalanceFromAccounts(resp models.AccountsResponse) (float64, bool) {
	for name, account := range resp.Accounts {
		if strings.EqualFold(name, "flex") {
			if v := account.BalanceValue.Float64(); v != 0 {
				return helpers.Round8(v), true
			}
			if v := account.PortfolioValue.Float64(); v != 0 {
				return helpers.Round8(v), true
			}
		}
	}
	for _, account := range resp.Accounts {
		if v := account.BalanceValue.Float64(); v != 0 {
			return helpers.Round8(v), true
		}
		if v := account.PortfolioValue.Float64(); v != 0 {
			return helpers.Round8(v), true
		}
	}
	return 0, false
}

func symbolFromPair(pair string) string {
	p := strings.ToUpper(strings.TrimSpace(pair))
	if strings.HasPrefix(p, "PF_") || strings.HasPrefix(p, "PI_") || strings.HasPrefix(p, "FI_") || strings.HasPrefix(p, "FF_") {
		return p
	}
	return "PF_" + p
}
