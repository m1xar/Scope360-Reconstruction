package models

type Candle struct {
	Time  int64   `json:"time"`
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
	Vol   float64 `json:"vol"`
}

// CandleColumnar is the raw columnar format returned by the MEXC kline API.
type CandleColumnar struct {
	Time  []int64   `json:"time"`
	Open  []float64 `json:"open"`
	Close []float64 `json:"close"`
	High  []float64 `json:"high"`
	Low   []float64 `json:"low"`
	Vol   []float64 `json:"vol"`
}

func (c *CandleColumnar) ToCandles() []Candle {
	n := len(c.Time)
	candles := make([]Candle, n)
	for i := 0; i < n; i++ {
		candles[i] = Candle{
			Time:  c.Time[i],
			Open:  c.Open[i],
			High:  c.High[i],
			Low:   c.Low[i],
			Close: c.Close[i],
			Vol:   c.Vol[i],
		}
	}
	return candles
}
