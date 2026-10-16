package fx

import (
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

const (
	Long  = "LONG"
	Short = "SHORT"
	Buy   = "BUY"
	Sell  = "SELL"

	OrderMarket = "MARKET"
	OrderLimit  = "LIMIT"
	OrderStop   = "STOP"

	StatusFilled          = "FILLED"
	StatusPartiallyFilled = "PARTIALLY_FILLED"
	StatusWorking         = "WORKING"
	StatusCanceled        = "CANCELED"

	StatusWin  = "win"
	StatusLose = "lose"
)

func NormalizePair(symbol string) string {
	s := strings.TrimSpace(symbol)
	if i := strings.Index(s, "."); i > 0 {
		s = s[:i]
	}
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func PositionSide(orderSide string) string {
	if strings.EqualFold(strings.TrimSpace(orderSide), Sell) {
		return Short
	}
	return Long
}

func OrderSide(positionSide string) string {
	if positionSide == Short {
		return Sell
	}
	return Buy
}

func ClosingSide(positionSide string) string {
	if positionSide == Short {
		return Buy
	}
	return Sell
}

func Status(netPnl float64) *string {
	status := StatusLose
	if netPnl > 0 {
		status = StatusWin
	}
	return &status
}

func PriceOrNil(price float64) *float64 {
	if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return nil
	}
	p := price
	return &p
}

func ApplyRR(pos *domain.FXPosition) {
	if pos == nil || pos.SL == nil {
		return
	}
	risk := math.Abs(pos.EntryPrice - *pos.SL)
	if risk == 0 {
		return
	}
	move := pos.ExitPrice - pos.EntryPrice
	if pos.Side == Short {
		move = -move
	}
	rr := round(move/risk, 4)
	pos.RR = &rr
	if pos.TP != nil {
		planned := round(math.Abs(*pos.TP-pos.EntryPrice)/risk, 4)
		pos.RRPlanned = &planned
	}
}

type Fill struct {
	Price    float64
	Quantity float64
}

func VWAP(fills []Fill) float64 {
	var notional, qty float64
	for _, f := range fills {
		notional += f.Price * f.Quantity
		qty += f.Quantity
	}
	if qty == 0 {
		return 0
	}
	return notional / qty
}

type BalanceEvent struct {
	Time  time.Time
	Delta float64
}

func ReverseSnapshots(current float64, events []BalanceEvent, windowStart *time.Time, now time.Time) []domain.UserBalanceSnapshot {
	sorted := append([]BalanceEvent(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Time.Before(sorted[j].Time) })

	type point struct {
		at      time.Time
		balance float64
	}
	points := make([]point, 0, len(sorted)+2)
	running := current
	for i := len(sorted) - 1; i >= 0; {
		at := sorted[i].Time
		after := running
		for i >= 0 && sorted[i].Time.Equal(at) {
			running -= sorted[i].Delta
			i--
		}
		points = append(points, point{at: at, balance: after})
	}
	start := running

	out := make([]domain.UserBalanceSnapshot, 0, len(points)+2)
	if windowStart != nil {

		atStart := start
		for _, p := range points {
			if !p.at.After(*windowStart) {
				atStart = p.balance
				break
			}
		}
		out = append(out, domain.UserBalanceSnapshot{CreatedAt: windowStart.UTC(), Balance: round(atStart, 8)})
	} else if len(points) > 0 {
		first := points[len(points)-1].at.Add(-time.Second)
		out = append(out, domain.UserBalanceSnapshot{CreatedAt: first.UTC(), Balance: round(start, 8)})
	}
	for i := len(points) - 1; i >= 0; i-- {
		p := points[i]
		if windowStart != nil && !p.at.After(*windowStart) {
			continue
		}
		out = append(out, domain.UserBalanceSnapshot{CreatedAt: p.at.UTC(), Balance: round(p.balance, 8)})
	}
	if len(out) == 0 || (now.After(out[len(out)-1].CreatedAt) && out[len(out)-1].Balance != round(current, 8)) {
		out = append(out, domain.UserBalanceSnapshot{CreatedAt: now.UTC(), Balance: round(current, 8)})
	}
	return out
}

func StartingBalance(current float64, events []BalanceEvent) float64 {
	for _, e := range events {
		current -= e.Delta
	}
	return current
}

func BalanceAt(snapshots []domain.UserBalanceSnapshot, t time.Time) float64 {
	if len(snapshots) == 0 {
		return 0
	}
	balance := snapshots[0].Balance
	for _, s := range snapshots {
		if s.CreatedAt.After(t) {
			break
		}
		balance = s.Balance
	}
	return balance
}

func AttachBalanceInit(positions []domain.FXPosition, snapshots []domain.UserBalanceSnapshot) {
	for i := range positions {
		positions[i].BalanceInit = BalanceAt(snapshots, positions[i].CreatedAt)
	}
}

func ClosedAfter(positions []domain.FXPosition, cutoff *time.Time) []domain.FXPosition {
	if cutoff == nil {
		return positions
	}
	kept := positions[:0]
	for _, p := range positions {
		if p.ClosedAt != nil && !p.ClosedAt.Before(*cutoff) {
			kept = append(kept, p)
		}
	}
	return kept
}

func CutoffFromDays(days int) *time.Time {
	if days <= 0 {
		return nil
	}
	t := time.Now().UTC().AddDate(0, 0, -days)
	return &t
}

func SortPositions(positions []domain.FXPosition) {
	sort.SliceStable(positions, func(i, j int) bool {
		a, b := positions[i], positions[j]
		if a.ClosedAt != nil && b.ClosedAt != nil && !a.ClosedAt.Equal(*b.ClosedAt) {
			return a.ClosedAt.Before(*b.ClosedAt)
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID < b.ID
	})
}

func round(v float64, digits int) float64 {
	p := math.Pow10(digits)
	return math.Round(v*p) / p
}

func Round8(v float64) float64 { return round(v, 8) }
