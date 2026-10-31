package builders

import (
	"sort"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
)

type ccyWallet struct {
	cash     float64
	isolated map[string]float64
}

func (w *ccyWallet) apply(b models.Bill) {
	w.cash = helpers.MustFloat(b.Bal)
	if b.MgnMode != "isolated" {
		return
	}
	if posBal := helpers.MustFloat(b.PosBal); posBal > 0 {
		w.isolated[b.InstId] = posBal
	} else {
		delete(w.isolated, b.InstId)
	}
}

func (w *ccyWallet) applyBefore(b models.Bill) {
	w.cash = helpers.MustFloat(b.Bal) - helpers.MustFloat(b.BalChg)
	if b.MgnMode != "isolated" {
		return
	}
	if posBal := helpers.MustFloat(b.PosBal) - helpers.MustFloat(b.PosBalChg); posBal > 0 {
		w.isolated[b.InstId] = posBal
	}
}

func (w *ccyWallet) total() float64 {
	total := w.cash
	for _, v := range w.isolated {
		total += v
	}
	return total
}

type wallets map[string]*ccyWallet

func (ws wallets) get(ccy string) *ccyWallet {
	w, ok := ws[ccy]
	if !ok {
		w = &ccyWallet{isolated: map[string]float64{}}
		ws[ccy] = w
	}
	return w
}

func (ws wallets) total() float64 {
	var sum float64
	for _, w := range ws {
		sum += w.total()
	}
	return sum
}

func BuildBalanceSnapshotsFromBills(
	currentBalance float64,
	bills []models.Bill,
) []domain.UserBalanceSnapshot {
	ordered := make([]models.Bill, 0, len(bills))
	for _, b := range bills {
		if helpers.IsStableAsset(b.Ccy) {
			ordered = append(ordered, b)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		ti, tj := helpers.MustInt64(ordered[i].Ts), helpers.MustInt64(ordered[j].Ts)
		if ti != tj {
			return ti < tj
		}
		return helpers.MustInt64(ordered[i].BillId) < helpers.MustInt64(ordered[j].BillId)
	})

	snapshots := make([]domain.UserBalanceSnapshot, 0, len(ordered)+2)
	ws := wallets{}
	for i, b := range ordered {
		w := ws.get(b.Ccy)
		if i == 0 {
			w.applyBefore(b)
			snapshots = append(snapshots, domain.UserBalanceSnapshot{
				CreatedAt: helpers.TimeFromMs(b.Ts).Add(-time.Millisecond),
				Balance:   helpers.Round8(ws.total()),
			})
		}
		w.apply(b)
		snapshots = append(snapshots, domain.UserBalanceSnapshot{
			CreatedAt: helpers.TimeFromMs(b.Ts),
			Balance:   helpers.Round8(ws.total()),
		})
	}

	snapshots = append(snapshots, domain.UserBalanceSnapshot{
		CreatedAt: time.Now().UTC(),
		Balance:   helpers.Round8(currentBalance),
	})

	sort.SliceStable(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	deduped := snapshots[:0]
	for i, s := range snapshots {
		if i > 0 && s.CreatedAt.Equal(snapshots[i-1].CreatedAt) {
			deduped[len(deduped)-1] = s
			continue
		}
		deduped = append(deduped, s)
	}

	return deduped
}
