package builders

import (
	"sort"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
)

func BuildFundings(logs []models.AccountLog, pairBySymbol map[string]string) []domain.UserFunding {
	type fundingBucket struct {
		pair string
		day  time.Time
	}

	grouped := make(map[fundingBucket]float64)
	for _, row := range logs {
		if !strings.EqualFold(row.Asset, "usd") || !row.RealizedFunding.Valid || row.RealizedFunding.Value == 0 {
			continue
		}
		at, err := helpers.ParseTime(row.Date)
		if err != nil {
			continue
		}
		day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
		pair := helpers.NormalizePair(row.Contract, pairBySymbol)
		grouped[fundingBucket{pair: pair, day: day}] += row.RealizedFunding.Value
	}

	out := make([]domain.UserFunding, 0, len(grouped))
	for bucket, amount := range grouped {
		out = append(out, domain.UserFunding{
			Pair:      bucket.pair,
			Amount:    helpers.Round8(amount),
			CreatedAt: bucket.day,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}
