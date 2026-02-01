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

	fills, err := hyperliquid.FetchAllFills(
		client,
		"https://api.hyperliquid.xyz/info",
		"0x5B7E4Dc30a929C577F5C0DC1fB8D3069966675d8",
	)
	if err != nil {
		panic(err)
	}

	fills = reconstructor.NormalizeFills(fills)

	tradeFills := make(chan []hyperliquid.RawFill)
	positions := make(chan domain.Position)

	go func() {
		reconstructor.ReconstructTrades(fills, tradeFills)
		close(tradeFills)
	}()

	reconstructor.StartPositionBuilders(tradeFills, positions, 8)

	for pos := range positions {
		fmt.Printf("TRADE %+v\n", pos)
	}
}
