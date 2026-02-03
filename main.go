package main

import (
	"fmt"
	"hyperliquid-trade-reconstructor/internal/hyperliquid/executors"
	"hyperliquid-trade-reconstructor/internal/reconstructor/builders"
	"hyperliquid-trade-reconstructor/internal/reconstructor/workers"
	"net/http"

	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/reconstructor"
)

func main() {
	client := http.DefaultClient
	endpoint := "https://api.hyperliquid.xyz/info"
	user := "0xbC4042D191153Bb5ca1b446C01433c261175A6eE" //0x5B7E4Dc30a929C577F5C0DC1fB8D3069966675d8

	fills, err := executors.FetchAllFills(client, endpoint, user)
	if err != nil {
		panic(err)
	}

	orders, err := executors.FetchHistoricalOrders(client, endpoint, user)
	if err != nil {

		panic(err)
	}

	raw_fundings, err := executors.FetchAllFunding(client, endpoint, user, 0)
	if err != nil {
		panic(err)
	}

	orderIdx := reconstructor.BuildOrderIndex(orders)
	fills = reconstructor.NormalizeFills(fills)

	envelopes := make(chan domain.TradeEnvelope)
	positions := make(chan domain.Position)

	go func() {
		reconstructor.ReconstructTrades(fills, raw_fundings, orderIdx, envelopes)
		close(envelopes)
	}()

	fundings := []domain.UserFunding{}

	for _, fund := range raw_fundings {
		fundings = append(fundings, builders.BuildUserFunding(fund))
	}

	workers.StartPositionBuilders(envelopes, positions, 8)

	for pos := range positions {
		dto := pos.ToDTO()
		fmt.Printf("TRADE %+v\n", dto)
	}

	for _, fund := range fundings {
		fmt.Printf("FUNDING %+v\n", fund)
	}

}
