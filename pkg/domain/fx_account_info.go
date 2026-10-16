package domain

type FXAccountInfo struct {
	Balance    float64
	Equity     float64
	Margin     float64
	FreeMargin float64
	Leverage   uint64
	Currency   string
	IsDemo     bool
}
