package builders

import (
	"math"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	connector "github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/service/reconstructor/helpers"
)

func EnrichPositionsWithCurrentRisk(client *connector.Client, positions *[]domain.Position) error {
	if positions == nil || len(*positions) == 0 {
		return nil
	}

	resp, err := executors.FetchPositionsSnapshot(client)
	if err != nil {
		return err
	}
	if resp == nil || len(resp.Rows) == 0 {
		return nil
	}

	type riskInfo struct {
		leverage float64
		liq      float64
	}
	byPair := make(map[string]riskInfo, len(resp.Rows))
	for _, row := range resp.Rows {
		pair := strings.ToUpper(strings.TrimSpace(helpers.NormalizeSymbol(row.Symbol)))
		byPair[pair] = riskInfo{
			leverage: row.Leverage,
			liq:      row.EstLiqPrice,
		}
	}

	for i := range *positions {
		pair := strings.ToUpper(strings.TrimSpace((*positions)[i].Pair))
		risk, ok := byPair[pair]
		if !ok {
			continue
		}
		if risk.leverage > 0 {
			(*positions)[i].Multiplier = uint32(math.Round(risk.leverage))
		}
		if risk.liq > 0 {
			(*positions)[i].LiquidationPrice = helpers.Round8(risk.liq)
		}
	}

	return nil
}
