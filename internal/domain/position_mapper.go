package domain

func (p Position) ToDTO() PositionDTO {
	var tp, sl float64
	var status string

	if p.TP != nil {
		tp = *p.TP
	}
	if p.SL != nil {
		sl = *p.SL
	}
	if p.Status != nil {
		status = *p.Status
	}

	return PositionDTO{
		ID:         p.ID.String(),
		Side:       p.Side,
		Pair:       p.Pair,
		Amount:     p.Amount,
		EntryPrice: p.EntryPrice,
		ExitPrice:  p.ExitPrice,
		Pnl:        p.Pnl,
		NetPnl:     p.NetPnl,
		Commission: p.Commission,
		TP:         tp,
		SL:         sl,
		Status:     status,
		CreatedAt:  p.CreatedAt,
		ClosedAt:   p.ClosedAt,
	}
}
