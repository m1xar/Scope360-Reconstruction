package builders

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
)

const sizeEpsilon = 1e-9

type fillPart struct {
	Fill models.Fill
	Size float64
	At   time.Time
	Sign float64
}

type episode struct {
	Symbol   string
	OpenSign float64
	Parts    []fillPart
	OpenAt   time.Time
	CloseAt  time.Time
	PeakSize float64
}

func BuildClosedPositions(
	fills []models.Fill,
	positionEvents []models.PositionEventElement,
	pairBySymbol map[string]string,
) ([]domain.Position, error) {
	episodes := buildEpisodes(fills)
	positions := make([]domain.Position, 0, len(episodes))
	metas := make([]episode, 0, len(episodes))

	for _, ep := range episodes {
		pos, err := buildPosition(ep, pairBySymbol)
		if err != nil {
			return nil, err
		}
		positions = append(positions, pos)
		metas = append(metas, ep)
	}

	enrichWithPositionEvents(positions, metas, positionEvents, pairBySymbol)

	sort.Slice(positions, func(i, j int) bool {
		if positions[i].ClosedAt == nil {
			return false
		}
		if positions[j].ClosedAt == nil {
			return true
		}
		return positions[i].ClosedAt.Before(*positions[j].ClosedAt)
	})
	return positions, nil
}

func buildEpisodes(fills []models.Fill) []episode {
	bySymbol := make(map[string][]models.Fill)
	for _, fill := range fills {
		bySymbol[strings.ToUpper(fill.Symbol)] = append(bySymbol[strings.ToUpper(fill.Symbol)], fill)
	}

	episodes := make([]episode, 0)
	for symbol, group := range bySymbol {
		sort.Slice(group, func(i, j int) bool {
			ti, _ := helpers.ParseTime(group[i].FillTime)
			tj, _ := helpers.ParseTime(group[j].FillTime)
			if ti.Equal(tj) {
				return group[i].FillID < group[j].FillID
			}
			return ti.Before(tj)
		})

		var current *episode
		net := 0.0

		for _, fill := range group {
			at, err := helpers.ParseTime(fill.FillTime)
			if err != nil {
				continue
			}
			sign := helpers.SideSign(fill.Side)
			remaining := fill.Size.Float64()
			if remaining <= 0 {
				continue
			}

			for remaining > sizeEpsilon {
				if current == nil {
					current = &episode{
						Symbol:   symbol,
						OpenSign: sign,
						OpenAt:   at,
					}
					net = 0
				}

				partSize := remaining
				if math.Abs(net) > sizeEpsilon && sign != current.OpenSign {
					partSize = math.Min(math.Abs(net), remaining)
				}

				current.Parts = append(current.Parts, fillPart{
					Fill: fill,
					Size: partSize,
					At:   at,
					Sign: sign,
				})

				net += sign * partSize
				if absNet := math.Abs(net); absNet > current.PeakSize {
					current.PeakSize = absNet
				}
				remaining = helpers.Round8(remaining - partSize)

				if math.Abs(net) < sizeEpsilon {
					current.CloseAt = at
					episodes = append(episodes, *current)
					current = nil
					net = 0
				}
			}
		}
	}

	return episodes
}

func buildPosition(ep episode, pairBySymbol map[string]string) (domain.Position, error) {
	posID, err := uuid.NewV7()
	if err != nil {
		return domain.Position{}, err
	}

	var openSize, openNotional, closeSize, closeNotional float64
	orders := make([]domain.Order, 0, len(ep.Parts))

	for _, part := range ep.Parts {
		price := part.Fill.Price.Float64()
		if part.Sign == ep.OpenSign {
			openSize += part.Size
			openNotional += part.Size * price
		} else {
			closeSize += part.Size
			closeNotional += part.Size * price
		}

		orderID, err := uuid.NewV7()
		if err != nil {
			return domain.Position{}, err
		}
		side := helpers.OrderSide(part.Fill.Side)
		doneAt := part.At
		amount := helpers.Round8(part.Size)
		avgPrice := helpers.Round8(price)

		orders = append(orders, domain.Order{
			ID:              orderID,
			PositionID:      posID,
			ExchangeOrderID: part.Fill.OrderID,
			Type:            helpers.OrderType(part.Fill.FillType),
			Status:          "FILLED",
			Side:            side,
			Amount:          amount,
			AmountFilled:    amount,
			AveragePrice:    avgPrice,
			OriginalPrice:   avgPrice,
			UpdatedAt:       doneAt,
			Trade: domain.Trade{
				OrderID: orderID,
				Side:    side,
				Price:   avgPrice,
				Amount:  amount,
				DoneAt:  doneAt,
			},
		})
	}

	entry := 0.0
	if openSize > 0 {
		entry = openNotional / openSize
	}
	exit := 0.0
	if closeSize > 0 {
		exit = closeNotional / closeSize
	}

	side := helpers.PositionSideFromSign(ep.OpenSign)
	pnl := (exit - entry) * ep.PeakSize
	if side == "SHORT" {
		pnl = (entry - exit) * ep.PeakSize
	}
	status := "lose"
	if pnl > 0 {
		status = "win"
	}
	closedAt := ep.CloseAt

	return domain.Position{
		ID:         posID,
		Side:       side,
		Pair:       helpers.NormalizePair(ep.Symbol, pairBySymbol),
		Amount:     helpers.Round8(ep.PeakSize),
		EntryPrice: helpers.Round8(entry),
		ExitPrice:  helpers.Round8(exit),
		Pnl:        helpers.Round8(pnl),
		NetPnl:     helpers.Round8(pnl),
		Closed:     true,
		Status:     &status,
		CreatedAt:  ep.OpenAt,
		ClosedAt:   &closedAt,
		Orders:     orders,
	}, nil
}

func enrichWithPositionEvents(
	positions []domain.Position,
	episodes []episode,
	events []models.PositionEventElement,
	pairBySymbol map[string]string,
) {
	for i := range positions {
		ep := episodes[i]
		fillIDs := make(map[string]struct{}, len(ep.Parts))
		for _, part := range ep.Parts {
			if part.Fill.FillID != "" {
				fillIDs[part.Fill.FillID] = struct{}{}
			}
		}

		var fee, pnl, funding float64
		matched := false
		for _, ev := range events {
			upd := ev.Event.PositionUpdate
			if !sameSymbol(upd.Tradeable, ep.Symbol, pairBySymbol) {
				continue
			}

			eventTime := eventTime(upd, ev)
			if _, ok := fillIDs[upd.ExecutionUID]; !ok && (eventTime.Before(ep.OpenAt) || eventTime.After(ep.CloseAt)) {
				continue
			}

			fee += math.Abs(upd.Fee.Float64())
			pnl += upd.RealizedPnL.Float64()
			funding += upd.RealizedFunding.Float64()
			matched = true
		}

		if !matched {
			continue
		}

		net := pnl - fee + funding
		status := "lose"
		if net > 0 {
			status = "win"
		}
		positions[i].Pnl = helpers.Round8(pnl)
		positions[i].Commission = helpers.Round8(fee)
		positions[i].Funding = helpers.Round8(funding)
		positions[i].NetPnl = helpers.Round8(net)
		positions[i].Status = &status
	}
}

func sameSymbol(eventSymbol, episodeSymbol string, pairBySymbol map[string]string) bool {
	if strings.EqualFold(eventSymbol, episodeSymbol) {
		return true
	}
	return helpers.NormalizePair(eventSymbol, pairBySymbol) == helpers.NormalizePair(episodeSymbol, pairBySymbol)
}

func eventTime(upd models.PositionUpdate, ev models.PositionEventElement) time.Time {
	ms := upd.Timestamp
	if ms == 0 {
		ms = upd.FillTime
	}
	if ms == 0 {
		ms = upd.FundingRealizationTime
	}
	if ms == 0 {
		ms = ev.Timestamp
	}
	return time.UnixMilli(ms).UTC()
}

func ApplyMAEMFE(pos *domain.Position, high, low *float64) {
	if high == nil || low == nil {
		return
	}
	entry := pos.EntryPrice
	amount := pos.Amount

	if pos.Side == "LONG" {
		maeVal := helpers.Round8((*low - entry) * amount)
		mfeVal := helpers.Round8((*high - entry) * amount)
		pos.MAE = &maeVal
		pos.MFE = &mfeVal
		return
	}

	maeVal := helpers.Round8((entry - *high) * amount)
	mfeVal := helpers.Round8((entry - *low) * amount)
	pos.MAE = &maeVal
	pos.MFE = &mfeVal
}

func FillSignature(fill models.Fill) string {
	if fill.FillID != "" {
		return fill.FillID
	}
	return fmt.Sprintf("%s|%s|%s|%.12f|%.12f", fill.Symbol, fill.OrderID, fill.FillTime, fill.Size.Float64(), fill.Price.Float64())
}
