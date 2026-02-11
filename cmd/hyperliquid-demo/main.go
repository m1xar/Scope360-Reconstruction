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
)

func main() {
	client := http.DefaultClient
	endpoint := "https://api.hyperliquid.xyz/info"
	user := "0x5B7E4Dc30a929C577F5C0DC1fB8D3069966675d8" // 0x5B7E4Dc30a929C577F5C0DC1fB8D3069966675d8  0xbC4042D191153Bb5ca1b446C01433c261175A6eE

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

	rawFundings, err := executors.FetchAllFunding(client, endpoint, user, 0)
	if err != nil {
		panic(err)
	}

	rawPortfolio, err := executors.FetchPortfolioState(client, endpoint, user)
	if err != nil {
		panic(err)
	}

	portfolio, err := helpers.NormalizePortfolio(rawPortfolio)
	if err != nil {
		panic(err)
	}

	orderIdx := helpers.BuildOrderIndex(orders)
	fills = helpers.NormalizeFills(fills)

	envelopes := make(chan models.TradeEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		reconstructor.ReconstructTrades(fills, rawFundings, orderIdx, client, endpoint, envelopes)
		close(envelopes)
	}()

	fundings := []domain.UserFunding{}

	for _, fund := range rawFundings {
		fundings = append(fundings, builders.BuildUserFunding(fund))
	}

	balancesnapshots := builders.BuildUserBalanceSnapshotsFromPortfolio(portfolio)

	workers.StartPositionBuilders(envelopes, positionsCh, 8)

	positionList := make([]domain.Position, 0)
	for pos := range positionsCh {
		positionList = append(positionList, pos)
	}

	builders.AttachBalanceInitToPositions(&positionList, balancesnapshots)

	for _, pos := range positionList {
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

	for _, snap := range balancesnapshots {
		fmt.Printf("BALANCE SNAPSHOT %+v\n", snap)
	}

	state, err := executors.FetchClearinghouseState(client, endpoint, user)
	if err != nil {
		fmt.Printf("CLEARINGHOUSE ERR %v\n", err)
	}

	openPositions, err := builders.BuildOpenPositionsFromClearinghouse(state, client, endpoint)
	if err != nil {
		fmt.Printf("Building Position Error %v\n", err)
	}

	for _, pos := range openPositions {
		fmt.Printf("OPEN POSITION %+v\n", pos)
	}
}
