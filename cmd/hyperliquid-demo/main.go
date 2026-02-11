package main

import (
	"fmt"
	"hyperliquid-trade-reconstructor/pkg/hyperliquid"
	"net/http"
)

func main() {
	client := http.DefaultClient
	endpoint := "https://api.hyperliquid.xyz/info"
	user := "0x5B7E4Dc30a929C577F5C0DC1fB8D3069966675d8" // 0x5B7E4Dc30a929C577F5C0DC1fB8D3069966675d8  0xbC4042D191153Bb5ca1b446C01433c261175A6eE

	testMessage := "Login nonce: 123456"
	addr, sig, ok, err := hyperliquid.ValidateWalletSubscription(testMessage)
	if err != nil {
		fmt.Printf("SIGN ERR %v\n", err)
	} else {
		fmt.Printf("SIGN CHECK address=%s ok=%v\n", addr, ok)
	}
	_ = sig

	positionList, err := hyperliquid.GetBuiltPositions(client, endpoint, user)
	if err != nil {
		panic(err)
	}

	fundings, err := hyperliquid.GetFundings(client, endpoint, user)
	if err != nil {
		panic(err)
	}

	balanceSnapshots, err := hyperliquid.GetBalanceSnapshots(client, endpoint, user)
	if err != nil {
		panic(err)
	}

	openPositions, err := hyperliquid.GetOpenPositions(client, endpoint, user)
	if err != nil {
		fmt.Printf("Building Position Error %v\n", err)
	}

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

	for _, snap := range balanceSnapshots {
		fmt.Printf("BALANCE SNAPSHOT %+v\n", snap)
	}

	for _, pos := range openPositions {
		fmt.Printf("OPEN POSITION %+v\n", pos)
	}
}
