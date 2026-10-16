package helpers

import (
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func MergeOrders(lists ...[]models.Order) []models.Order {
	byID := make(map[string]int)
	var out []models.Order
	for _, list := range lists {
		for _, o := range list {
			id := o.Key()
			if i, ok := byID[id]; ok {
				if o.Newer(out[i]) {
					out[i] = o
				}
				continue
			}
			byID[id] = len(out)
			out = append(out, o)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].IssueTime.Before(out[j].IssueTime.Time) })
	return out
}

func MissingOpeners(history []models.Order) []string {
	have := make(map[string]bool)
	for _, o := range history {
		if !o.Closing() {
			have[o.PositionCode()] = true
		}
	}
	seen := make(map[string]bool)
	var out []string
	for _, o := range history {
		code := o.PositionCode()
		if !o.Closing() || code == "" || have[code] || seen[code] || len(Fills(o)) == 0 {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	return out
}

func Symbols(positions []models.Position) []string {
	seen := make(map[string]bool, len(positions))
	var out []string
	for _, p := range positions {
		if !seen[p.Symbol] {
			seen[p.Symbol] = true
			out = append(out, p.Symbol)
		}
	}
	return out
}

func OrderStatus(o models.Order) string {
	filled := o.Leg().FilledQuantity.Float() > Epsilon || len(Fills(o)) > 0
	switch o.Status {
	case models.StatusCompleted:
		return fx.StatusFilled
	case models.StatusCanceled, models.StatusExpired:
		if filled {
			return fx.StatusPartiallyFilled
		}
		if o.Status == models.StatusCanceled {
			return fx.StatusCanceled
		}
		return o.Status
	case models.StatusWorking, models.StatusAccepted:
		if filled {
			return fx.StatusPartiallyFilled
		}
		return fx.StatusWorking
	}
	return o.Status
}

func Final(o models.Order) bool {
	if o.FinalStatus {
		return true
	}
	switch o.Status {
	case models.StatusCompleted, models.StatusCanceled, models.StatusExpired, models.StatusRejected:
		return true
	}
	return false
}
