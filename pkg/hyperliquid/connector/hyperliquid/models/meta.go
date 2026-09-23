package models

type MetaResponse struct {
	Universe []MetaAsset `json:"universe"`
}

type MetaAsset struct {
	Name         string `json:"name"`
	SzDecimals   int    `json:"szDecimals"`
	MaxLeverage  int    `json:"maxLeverage"`
	OnlyIsolated bool   `json:"onlyIsolated"`
	IsDelisted   bool   `json:"isDelisted"`
}
