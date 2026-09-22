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
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

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

	return reconstructor.ReconstructClosedPositions(client, helpers.CutoffFromDays(days))
}

func GetClosedPositionByExactMatch(
	client *resty.Client,
	creds krakenclient.Credentials,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	positions, err := GetBuiltPositions(client, creds, window.DaysSince(openedAt))
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
	reconstructor.EnrichOpenPositionOrders(client, rawPositions, out)
	return out, nil
}

func GetBalanceSnapshots(
	client *resty.Client,
	creds krakenclient.Credentials,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	client = authClient(client, creds)
	return balanceSnapshotsFromClient(client, days)
}

// balanceSnapshotsFromClient assumes client already has auth attached.
// Used to avoid resty "Overwriting an existing pre-request hook" when callers
// fall back into snapshot fetch on an already-authed client.
func balanceSnapshotsFromClient(client *resty.Client, days int) ([]domain.UserBalanceSnapshot, error) {
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
		if val, ok := helpers.CurrentBalanceFromAccounts(accounts); ok {
			return &val, nil
		}
	}

	snapshots, snapErr := balanceSnapshotsFromClient(client, 0)
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

func GetTransactions(
	client *resty.Client,
	creds krakenclient.Credentials,
	days int,
) ([]domain.Transaction, error) {
	client = authClient(client, creds)

	logs, err := executors.FetchAllAccountLog(client, days)
	if err != nil {
		return nil, err
	}

	transactions := builders.BuildTransactions(logs)
	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		filtered := transactions[:0]
		for _, tx := range transactions {
			if !tx.Time.Before(*cutoff) {
				filtered = append(filtered, tx)
			}
		}
		transactions = filtered
	}
	return transactions, nil
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
	pairBySymbol := reconstructor.BuildPairMap(client, helpers.SymbolsFromAccountLogs(logs))
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
