package models

type AlgoOrder struct {
	AlgoId      string `json:"algoId"`
	InstId      string `json:"instId"`
	PosSide     string `json:"posSide"`
	TdMode      string `json:"tdMode"`
	TpTriggerPx string `json:"tpTriggerPx"`
	TpOrdPx     string `json:"tpOrdPx"`
	SlTriggerPx string `json:"slTriggerPx"`
	SlOrdPx     string `json:"slOrdPx"`
	CTime       string `json:"cTime"`
}
