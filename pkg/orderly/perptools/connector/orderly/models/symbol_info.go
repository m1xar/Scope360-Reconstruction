package models

type OrderlySymbolsInfoResponse struct {
	Rows []OrderlySymbolInfo `json:"rows"`
}

type OrderlySymbolInfo struct {
	Symbol      string `json:"symbol"`
	Base        string `json:"base"`
	Quote       string `json:"quote"`
	CreatedTime int64  `json:"created_time"`
}
