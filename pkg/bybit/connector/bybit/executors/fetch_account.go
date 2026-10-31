package executors

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/models"
)

const (
	accountInfoPath   = "/v5/account/info"
	walletBalancePath = "/v5/account/wallet-balance"

	accountTypeUnified = "UNIFIED"
)

var StableAssets = map[string]struct{}{
	"USDT": {},
	"USDC": {},
	"USDE": {},
}

func IsStableAsset(asset string) bool {
	_, ok := StableAssets[strings.ToUpper(strings.TrimSpace(asset))]
	return ok
}

func FetchAccountInfo(client *resty.Client) (models.AccountInfo, error) {
	info, err := doWithRateLimit(func() (models.AccountInfo, error) {
		return bybit.DoGet[models.AccountInfo](client, accountInfoPath, nil)
	})
	if err != nil {
		return info, err
	}
	if info.UnifiedMarginStatus == bybit.UnifiedStatusClassic {
		return info, fmt.Errorf("bybit: classic account is not supported")
	}
	return info, nil
}

func FetchWalletBalance(client *resty.Client) (models.WalletAccount, error) {
	wallet, err := doWithRateLimit(func() (models.WalletBalance, error) {
		return bybit.DoGet[models.WalletBalance](client, walletBalancePath, map[string]string{
			"accountType": accountTypeUnified,
		})
	})
	if err != nil {
		return models.WalletAccount{}, err
	}
	if len(wallet.List) == 0 {
		return models.WalletAccount{AccountType: accountTypeUnified}, nil
	}
	return wallet.List[0], nil
}

func parse(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func TotalWalletBalance(w models.WalletAccount) float64 {
	return parse(w.TotalWalletBalance)
}

func StableWalletBalance(w models.WalletAccount) float64 {
	var total float64
	for _, c := range w.Coin {
		if IsStableAsset(c.Coin) {
			total += parse(c.WalletBalance)
		}
	}
	return total
}
