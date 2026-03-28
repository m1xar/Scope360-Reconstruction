package models

type AlgoOrder struct {
	AlgoId      string `json:"algoId"`
	InstId      string `json:"instId"`
	InstType    string `json:"instType"`
	OrdType     string `json:"ordType"`
	PosSide     string `json:"posSide"`
	Side        string `json:"side"`
	State       string `json:"state"`
	SlTriggerPx string `json:"slTriggerPx"`
	SlOrdPx     string `json:"slOrdPx"`
	TpTriggerPx string `json:"tpTriggerPx"`
	TpOrdPx     string `json:"tpOrdPx"`
	Sz          string `json:"sz"`
	CTime       string `json:"cTime"`
	TriggerTime string `json:"triggerTime"`
}
