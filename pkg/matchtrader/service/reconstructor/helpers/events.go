package helpers

import (
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
)

func GroupClosed(events []models.ClosedPosition) map[string][]models.ClosedPosition {
	out := make(map[string][]models.ClosedPosition)
	for _, e := range events {
		if e.ID == "" {
			continue
		}
		out[e.ID] = append(out[e.ID], e)
	}
	for _, list := range out {
		sort.SliceStable(list, func(i, j int) bool { return list[i].ClosedAt().Before(list[j].ClosedAt()) })
	}
	return out
}
