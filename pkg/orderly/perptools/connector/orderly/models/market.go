package models

type OrderlyFuturesResponse struct {
	Rows []OrderlyFutureMarket `json:"rows"`
}

type OrderlyFutureMarket struct {
	Symbol            string  `json:"symbol"`
	DisplaySymbolName string  `json:"display_symbol_name"`
	MarkPrice         float64 `json:"mark_price"`
	IndexPrice        float64 `json:"index_price"`
}
