package models

type HistoryPosition struct {
	PositionId   int64   `json:"positionId"`
	Symbol       string  `json:"symbol"`
	PositionType int     `json:"positionType"` // 1=long, 2=short
	OpenType     int     `json:"openType"`     // 1=isolated, 2=cross
	State        int     `json:"state"`
	HoldVol      float64 `json:"holdVol"`
	FrozenVol    float64 `json:"frozenVol"`
	CloseVol     float64 `json:"closeVol"`
	HoldAvgPrice float64 `json:"holdAvgPrice"`
	OpenAvgPrice float64 `json:"openAvgPrice"`
	CloseAvgPrice float64 `json:"closeAvgPrice"`
	LiquidatePrice float64 `json:"liquidatePrice"`
	Oim          float64 `json:"oim"`
	Im           float64 `json:"im"`
	HoldFee      float64 `json:"holdFee"`
	Realised     float64 `json:"realised"`
	Leverage     int     `json:"leverage"`
	CreateTime   int64   `json:"createTime"`
	UpdateTime   int64   `json:"updateTime"`
	AutoAddIm    bool    `json:"autoAddIm"`
	AdlLevel     int     `json:"adlLevel"`
}

type OpenPosition struct {
	PositionId     int64   `json:"positionId"`
	Symbol         string  `json:"symbol"`
	HoldVol        float64 `json:"holdVol"`
	PositionType   int     `json:"positionType"`
	OpenType       int     `json:"openType"`
	State          int     `json:"state"`
	FrozenVol      float64 `json:"frozenVol"`
	CloseVol       float64 `json:"closeVol"`
	HoldAvgPrice   float64 `json:"holdAvgPrice"`
	OpenAvgPrice   float64 `json:"openAvgPrice"`
	CloseAvgPrice  float64 `json:"closeAvgPrice"`
	LiquidatePrice float64 `json:"liquidatePrice"`
	Oim            float64 `json:"oim"`
	Im             float64 `json:"im"`
	HoldFee        float64 `json:"holdFee"`
	Realised       float64 `json:"realised"`
	Leverage       int     `json:"leverage"`
	CreateTime     int64   `json:"createTime"`
	UpdateTime     int64   `json:"updateTime"`
	AdlLevel       int     `json:"adlLevel"`
}
