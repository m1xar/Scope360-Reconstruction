package okx

import (
	"fmt"

	"github.com/go-resty/resty/v2"
)

type Region string

const (
	RegionGlobal Region = "global"
	RegionUS     Region = "us"
	RegionEEA    Region = "eea"
)

func BaseURL(r Region) string {
	switch r {
	case RegionUS:
		return "https://us.okx.com"
	case RegionEEA:
		return "https://eea.okx.com"
	default:
		return "https://www.okx.com"
	}
}

var allRegions = []Region{RegionGlobal, RegionEEA, RegionUS}

func CheckAccount(apiKey, secret, passphrase string) (Region, error) {
	client := NewBaseClient()
	AttachAuth(client, Credentials{
		APIKey:     apiKey,
		Secret:     secret,
		Passphrase: passphrase,
	})

	for _, region := range allRegions {
		if tryBalance(client, BaseURL(region)) {
			return region, nil
		}
	}

	return "", fmt.Errorf("okx: invalid credentials or account not found in any region")
}

func tryBalance(client *resty.Client, baseURL string) bool {
	type balance struct {
		TotalEq string `json:"totalEq"`
	}
	_, err := DoGet[[]balance](client, baseURL, "/api/v5/account/balance", nil)
	return err == nil
}
