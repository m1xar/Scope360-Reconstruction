package executors

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

func FetchAllCandlesHyperliquid(
	client *resty.Client,
	endpoint string,
	coin string,
	interval string,
	startTime int64,
	endTime int64,
) ([]models.HyperliquidCandle, error) {

	type candleRequest struct {
		Type string `json:"type"`
		Req  struct {
			Coin      string `json:"coin"`
			Interval  string `json:"interval"`
			StartTime int64  `json:"startTime"`
			EndTime   int64  `json:"endTime"`
		} `json:"req"`
	}

	payload := candleRequest{Type: "candleSnapshot"}
	payload.Req.Coin = coin
	payload.Req.Interval = interval
	payload.Req.StartTime = startTime
	payload.Req.EndTime = endTime

	var result []models.HyperliquidCandle
	err := hyperliquid.DoRequest(client, endpoint, payload, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
