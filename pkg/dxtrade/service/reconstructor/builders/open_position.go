package builders

import (
	"math"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func BuildOpenPositions(
	positions []models.Position,
	orders []models.Order,
	ledger []helpers.CashEntry,
	metrics *models.AccountMetrics,
	quotes map[string]models.Quote,
	lotSizes map[string]float64,
	hedging bool,
) []domain.FXOpenPosition {
	fpl := make(map[string]models.PositionMetrics)
	if metrics != nil {
		for _, m := range metrics.Positions {
			fpl[m.PositionCode.String()] = m
		}
	}
	cash := helpers.ByPosition(ledger)
	out := make([]domain.FXOpenPosition, 0, len(positions))
	for _, p := range positions {
		code := p.PositionCode.String()
		sums := helpers.SumEntries(cash[code])
		side := fx.PositionSide(p.Side)
		units := math.Abs(p.Quantity.Float())
		amount := helpers.Lots(lotSizes, p.Symbol, units)
		entry := p.OpenPrice.Float()
		var related []models.Order
		for _, o := range orders {
			if hedging && o.PositionCode() == code {
				related = append(related, o)
			}
			if !hedging && o.Symbol() == p.Symbol && !o.TransactionTime.Before(p.OpenTime.Time) {
				related = append(related, o)
			}
		}
		built := BuildOrders(related, code, lotSizes)
		for i := range built {
			if built[i].Amount == 0 && built[i].Status == fx.StatusWorking {
				built[i].Amount = amount
			}
		}
		built = append(built, attachedProtection(p, related, code, amount)...)
		out = append(out, domain.FXOpenPosition{
			ID:            code,
			Pair:          fx.NormalizePair(p.Symbol),
			Symbol:        p.Symbol,
			Amount:        amount,
			Side:          side,
			EntryPrice:    entry,
			CurrentPrice:  currentPrice(side, entry, units, quotes[p.Symbol], fpl[code]),
			UnrealizedPnl: fpl[code].Fpl.Float(),
			Commission:    fx.Round8(math.Abs(sums.Commission)),
			Swap:          fx.Round8(sums.Financing),
			Margin:        fpl[code].Margin.Float(),
			OpenTime:      p.OpenTime.Time,
			Orders:        built,
			ContractSize:  helpers.ContractSize(lotSizes, p.Symbol),
		})
	}
	return out
}

func currentPrice(side string, entry, amount float64, quote models.Quote, m models.PositionMetrics) float64 {
	if rate := m.Rate.Float(); rate > 0 {
		return rate
	}
	if side == fx.Long && quote.Bid > 0 {
		return quote.Bid
	}
	if side == fx.Short && quote.Ask > 0 {
		return quote.Ask
	}
	if amount <= 0 || (m.PositionCode == "" && m.Fpl == 0) {
		return 0
	}
	conv := m.ConvRate.Float()
	if conv <= 0 {
		conv = 1
	}
	move := m.Fpl.Float() / (amount * conv)
	if side == fx.Short {
		move = -move
	}
	return entry + move
}

func attachedProtection(p models.Position, orders []models.Order, code string, amount float64) []domain.FXOrder {
	hasTP, hasSL := false, false
	for _, o := range orders {
		if !o.Closing() || helpers.Final(o) {
			continue
		}
		switch o.Type {
		case fx.OrderLimit:
			hasTP = true
		case fx.OrderStop:
			hasSL = true
		}
	}
	side := fx.ClosingSide(fx.PositionSide(p.Side))
	var out []domain.FXOrder
	if tp := p.TakeProfitPrice.Float(); tp > 0 && !hasTP {
		out = append(out, domain.FXOrder{
			ID: code + ":tp", PositionID: code, Type: fx.OrderLimit, Status: fx.StatusWorking,
			Side: side, Amount: amount, OriginalPrice: tp, UpdatedAt: p.LastUpdateTime.Time,
		})
	}
	if sl := p.StopLossPrice.Float(); sl > 0 && !hasSL {
		out = append(out, domain.FXOrder{
			ID: code + ":sl", PositionID: code, Type: fx.OrderStop, Status: fx.StatusWorking,
			Side: side, Amount: amount, StopPrice: sl, OriginalPrice: sl, UpdatedAt: p.LastUpdateTime.Time,
		})
	}
	return out
}
