package helpers

import (
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	connector "github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func Leverage(s *connector.Session) uint64 {
	if s == nil {
		return 0
	}
	lev := s.Account.Offer.Leverage.Float()
	if lev <= 0 {
		lev = s.Account.Leverage.Float()
	}
	if lev <= 0 {
		return 0
	}
	return uint64(lev)
}

func AccountInfo(s *connector.Session, b *models.Balance) domain.FXAccountInfo {
	info := domain.FXAccountInfo{Leverage: Leverage(s)}
	if s != nil {
		info.Currency = s.Account.Offer.Currency
		info.IsDemo = s.Account.Offer.Demo || s.Account.Offer.System.Demo
	}
	if b != nil {
		info.Balance = fx.Round8(b.Balance.Float())
		info.Equity = fx.Round8(b.Equity.Float())
		info.Margin = fx.Round8(b.Margin.Float())
		info.FreeMargin = fx.Round8(b.FreeMargin.Float())
		if b.Currency != "" {
			info.Currency = b.Currency
		}
	}
	return info
}
