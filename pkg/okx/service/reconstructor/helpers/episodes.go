package helpers

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

const sizeEpsilon = 1e-9

type FillPart struct {
	Fill  models.Fill
	Size  float64
	Price float64
	At    time.Time
	Sign  float64
}

type Episode struct {
	InstID   string
	OpenSign float64
	Parts    []FillPart
	OpenAt   time.Time
	CloseAt  time.Time
	PeakSize float64
	Closed   bool
}

func PositionKey(instID, posSide string) string {
	side := strings.ToLower(strings.TrimSpace(posSide))
	if side == "" || side == "net" {
		return instID
	}
	return instID + "|" + side
}

func BuildEpisodesFromGroups(
	groups [][]models.Fill,
	instruments map[string]models.Instrument,
) []Episode {
	episodes := make([]Episode, 0, len(groups))
	for _, group := range groups {
		episodes = append(episodes, accumulateEpisodes(group, instruments)...)
	}

	sort.SliceStable(episodes, func(i, j int) bool {
		return episodes[i].CloseAt.Before(episodes[j].CloseAt)
	})

	return episodes
}

func PartsFromFills(fills []models.Fill, instruments map[string]models.Instrument) []FillPart {
	parts := make([]FillPart, 0, len(fills))
	for _, fill := range sortedFills(fills) {
		size := ContractsToBase(fill.FillSz, InstrumentFor(instruments, fill.InstId))
		if size <= sizeEpsilon {
			continue
		}
		parts = append(parts, FillPart{
			Fill:  fill,
			Size:  size,
			Price: MustFloat(fill.FillPx),
			At:    TimeFromMs(fill.Ts),
			Sign:  fillSign(fill),
		})
	}
	return parts
}

func sortedFills(group []models.Fill) []models.Fill {
	sorted := make([]models.Fill, len(group))
	copy(sorted, group)
	sort.SliceStable(sorted, func(i, j int) bool {
		ti, tj := MustInt64(sorted[i].Ts), MustInt64(sorted[j].Ts)
		if ti == tj {
			return MustInt64(sorted[i].BillId) < MustInt64(sorted[j].BillId)
		}
		return ti < tj
	})
	return sorted
}

func accumulateEpisodes(group []models.Fill, instruments map[string]models.Instrument) []Episode {
	episodes := make([]Episode, 0, 1)
	var current *Episode
	net := 0.0

	for _, part := range PartsFromFills(group, instruments) {
		remaining := part.Size
		for remaining > sizeEpsilon {
			if current == nil {
				current = &Episode{
					InstID:   part.Fill.InstId,
					OpenSign: part.Sign,
					OpenAt:   part.At,
				}
				net = 0
			}

			partSize := remaining
			if math.Abs(net) > sizeEpsilon && part.Sign != current.OpenSign {
				partSize = math.Min(math.Abs(net), remaining)
			}

			current.Parts = append(current.Parts, FillPart{
				Fill:  part.Fill,
				Size:  partSize,
				Price: part.Price,
				At:    part.At,
				Sign:  part.Sign,
			})

			net += part.Sign * partSize
			if absNet := math.Abs(net); absNet > current.PeakSize {
				current.PeakSize = absNet
			}
			remaining = Round8(remaining - partSize)

			if math.Abs(net) < sizeEpsilon {
				current.CloseAt = part.At
				current.Closed = true
				episodes = append(episodes, *current)
				current = nil
				net = 0
			}
		}
	}

	if current != nil {
		current.CloseAt = current.Parts[len(current.Parts)-1].At
		episodes = append(episodes, *current)
	}

	return episodes
}

func (e Episode) Side() string {
	if e.OpenSign < 0 {
		return "SHORT"
	}
	return "LONG"
}

func fillSign(fill models.Fill) float64 {
	if strings.EqualFold(strings.TrimSpace(fill.Side), "sell") {
		return -1
	}
	return 1
}

func InstrumentFor(instruments map[string]models.Instrument, instID string) models.Instrument {
	if inst, ok := instruments[instID]; ok && inst.CtVal != "" {
		return inst
	}
	return models.DefaultInstrument
}
