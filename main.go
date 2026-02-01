package main

import (
	"fmt"
	"net/http"

	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/hyperliquid"
	"hyperliquid-trade-reconstructor/internal/reconstructor"
)

func main() {
	client := http.DefaultClient
	endpoint := "https://api.hyperliquid.xyz/info"
	user := "0x5B7E4Dc30a929C577F5C0DC1fB8D3069966675d8"

	fills, err := hyperliquid.FetchAllFills(client, endpoint, user)
	if err != nil {
		panic(err)
	}

	orders, err := hyperliquid.FetchHistoricalOrders(client, endpoint, user)
	if err != nil {
		panic(err)
	}

	orderIdx := reconstructor.BuildOrderIndex(orders)
	fills = reconstructor.NormalizeFills(fills)

	trades := make(chan reconstructor.TradeEnvelope)
	positions := make(chan domain.Position)

	go func() {
		reconstructor.ReconstructTrades(fills, orderIdx, trades)
		close(trades)
	}()

	reconstructor.StartPositionBuilders(trades, positions, 8)

	for pos := range positions {
		dto := pos.ToDTO()
		fmt.Printf("TRADE %+v\n", dto)
	}
}
