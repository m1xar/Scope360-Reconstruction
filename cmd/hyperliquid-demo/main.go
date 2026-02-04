package main

import (
	"fmt"
	"hyperliquid-trade-reconstructor/internal/connector/hyperliquid/executors"
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/builders"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/helpers"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/models"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/workers"
	"net/http"
	"time"
)

func main() {
	client := http.DefaultClient
	endpoint := "https://api.hyperliquid.xyz/info"
	user := "0xbC4042D191153Bb5ca1b446C01433c261175A6eE" // 0x5B7E4Dc30a929C577F5C0DC1fB8D3069966675d8

	testMessage := "Login nonce: 123456"
	addr, sig, err := helpers.CreateWalletAndSign(testMessage)
	if err != nil {
		fmt.Printf("SIGN ERR %v\n", err)
	} else {
		ok := helpers.VerifySignature(addr, sig, testMessage)
		fmt.Printf("SIGN CHECK address=%s ok=%v\n", addr, ok)
	}

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

	orderIdx := helpers.BuildOrderIndex(orders)
	fills = helpers.NormalizeFills(fills)

	envelopes := make(chan models.TradeEnvelope)
	positions := make(chan domain.Position)

	go func() {
		reconstructor.ReconstructTrades(fills, raw_fundings, orderIdx, client, endpoint, envelopes)
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
		for _, order := range pos.Orders {
			fmt.Printf("Order %+v\n", order)
		}
		fmt.Printf("\n")
	}

	for _, fund := range fundings {
		fmt.Printf("FUNDING %+v\n", fund)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		state, err := executors.FetchClearinghouseState(client, endpoint, user)
		if err != nil {
			fmt.Printf("CLEARINGHOUSE ERR %v\n", err)
			continue
		}

		openPositions, err := builders.BuildOpenPositionsFromClearinghouse(state, client, endpoint)
		if err != nil {
			fmt.Printf("Building Position Error %v\n", err)
			continue
		}

		for _, pos := range openPositions {
			fmt.Printf("OPEN POSITION %+v\n", pos)
		}
	}
}
