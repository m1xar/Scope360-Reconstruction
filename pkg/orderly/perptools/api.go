package perptools

import (
	"errors"
	"time"

	"github.com/go-resty/resty/v2"
	connector "github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/models"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/helpers"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

func newClient(httpClient *resty.Client, cfg connector.Config) *connector.Client {
	cfg.HTTPClient = httpClient
	return connector.NewClient(cfg)
}

func load(client *resty.Client, cfg connector.Config, days int, s scope.Scope) (*reconstructor.Dataset, error) {
	return reconstructor.Load(newClient(client, cfg), helpers.CutoffFromDays(days), s)
}

// Sync fetches the account's raw data once and builds every model from it:
// closed and open positions, balance snapshots, the current balance,
// transactions and fundings for the last days (the whole history when
// days <= 0).
func Sync(client *resty.Client, cfg connector.Config, days int) (*domain.Sync, error) {
	d, err := load(client, cfg, days, scope.All)
	if err != nil {
		return nil, err
	}

	out := &domain.Sync{}
	err = parallel.Run(
		func() (err error) { out.Positions, err = d.ClosedPositions(); return err },
		func() (err error) { out.OpenPositions, err = d.OpenPositions(); return err },
		func() (err error) { out.BalanceSnapshots, err = d.BalanceSnapshots(); return err },
		func() (err error) { out.Transactions, err = d.Transactions(); return err },
		func() error { out.Fundings = d.Fundings(); return nil },
	)
	if err != nil {
		return nil, err
	}
	out.CurrentBalance = d.CurrentBalance()
	return out, nil
}

func GetBuiltPositions(client *resty.Client, cfg connector.Config, days int) ([]domain.Position, error) {
	// Balances joins the scope for BalanceInit.
	d, err := load(client, cfg, days, scope.Closed|scope.Balances)
	if err != nil {
		return nil, err
	}
	return d.ClosedPositions()
}

func GetClosedPositionByExactMatch(
	client *resty.Client,
	cfg connector.Config,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	positions, err := GetBuiltPositions(client, cfg, window.DaysSince(openedAt))
	if err != nil {
		return nil, err
	}

	normalizedPair := helpers.NormalizeSymbol(helpers.SymbolFromPair(pair))
	for _, pos := range positions {
		if pos.Pair == normalizedPair && pos.Side == side && pos.CreatedAt.Equal(openedAt) {
			matched := pos
			return &matched, nil
		}
	}

	return nil, nil
}

func GetBalanceSnapshots(client *resty.Client, cfg connector.Config, days int) ([]domain.UserBalanceSnapshot, error) {
	d, err := load(client, cfg, days, scope.Balances)
	if err != nil {
		return nil, err
	}
	return d.BalanceSnapshots()
}

func GetCurrentBalance(client *resty.Client, cfg connector.Config) (*float64, error) {
	c := newClient(client, cfg)

	snapshot, err := executors.FetchPositionsSnapshot(c)
	if err != nil {
		return nil, err
	}

	balance := helpers.Round8(snapshot.AccountValue)
	return &balance, nil
}

func GetTransactions(client *resty.Client, cfg connector.Config, days int) ([]domain.Transaction, error) {
	d, err := load(client, cfg, days, scope.Transactions)
	if err != nil {
		return nil, err
	}
	return d.Transactions()
}

func GetFundings(client *resty.Client, cfg connector.Config, days int) ([]domain.UserFunding, error) {
	d, err := load(client, cfg, days, scope.Fundings)
	if err != nil {
		return nil, err
	}
	return d.Fundings(), nil
}

func GetCandles(
	client *resty.Client,
	cfg connector.Config,
	coin string,
	interval string,
	startTime time.Time,
	endTime time.Time,
) ([]models.OrderlyCandle, error) {
	c := newClient(client, cfg)

	if endTime.Before(startTime) {
		return nil, errors.New("endTime must be >= startTime")
	}

	symbol := "PERP_" + coin + "_USDC"
	startMs := startTime.UnixMilli()
	endMs := endTime.UnixMilli()

	candles, err := executors.FetchCandles(c, symbol, interval, startMs, endMs)
	if err != nil {
		return nil, err
	}

	return candles, nil
}

func GetOpenPositions(client *resty.Client, cfg connector.Config) ([]domain.OpenPosition, error) {
	d, err := load(client, cfg, 0, scope.Open)
	if err != nil {
		return nil, err
	}
	return d.OpenPositions()
}

func ValidateWalletSubscription(address, signature, message string) (bool, error) {
	ok := connector.VerifyWalletSignature(address, signature, message)
	return ok, nil
}
