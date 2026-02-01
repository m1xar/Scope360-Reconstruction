package reconstructor

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/hyperliquid"
	"time"

	"github.com/google/uuid"
)

func BuildPositionFromFills(fills []hyperliquid.RawFill) domain.Position {
	first := fills[0]
	last := fills[len(fills)-1]

	var amount, fee, pnl float64

	for _, f := range fills {
		if isOpen(f.Dir) {
			amount += mustFloat(f.Sz)
		}
		fee += mustFloat(f.Fee)
		pnl += mustFloat(f.ClosedPnl)
	}

	entry := mustFloat(first.Px)
	exit := mustFloat(last.Px)

	start := time.UnixMilli(first.Time)
	end := time.UnixMilli(last.Time)

	net := pnl - fee
	status := "Loss"
	if net > 0 {
		status = "Win"
	}

	return domain.Position{
		ID:         uuid.New(),
		UserID:     uint64(uuid.New().ID()),
		KeyID:      uint64(uuid.New().ID()),
		Side:       sideFromDir(first.Dir),
		Pair:       first.Coin + "/" + first.FeeToken,
		Amount:     amount,
		EntryPrice: entry,
		ExitPrice:  exit,
		Pnl:        pnl,
		NetPnl:     net,
		Commission: fee,
		Funding:    0,
		MAE:        0,
		MFE:        0,
		Isolated:   true,
		Closed:     true,
		Status:     &status,
		CreatedAt:  start,
		ClosedAt:   &end,
		Orders:     []domain.Order{},
		Editable:   domain.JSONMap{"editable": true},
	}
}
