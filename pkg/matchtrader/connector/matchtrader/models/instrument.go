package models

import "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"

type Instrument struct {
	Symbol          string    `json:"symbol"`
	Alias           string    `json:"alias"`
	Type            string    `json:"type"`
	ContractSize    fx.Number `json:"contractSize"`
	PricePrecision  int       `json:"pricePrecision"`
	VolumePrecision int       `json:"volumePrecision"`
	SizeOfOnePoint  fx.Number `json:"sizeOfOnePoint"`
	BaseCurrency    string    `json:"baseCurrency"`
	QuoteCurrency   string    `json:"quoteCurrency"`
}
