package fx

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/candlespan"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/excursion"
)

type Bar struct {
	OpenTime time.Time
	High     float64
	Low      float64
}

type BarFetcher func(symbol, interval string, from, to time.Time) ([]Bar, error)

type leg struct {
	at       time.Time
	quantity float64
	price    float64
	opening  bool
}

type Exposure struct {
	Symbol   string
	Side     string
	Orders   []domain.FXOrder
	End      time.Time
	EndPrice float64
	Gross    float64
	Net      float64
}

func EnrichMAEMFE(positions []domain.FXPosition, fetch BarFetcher, workers int) {
	if fetch == nil {
		return
	}
	run(len(positions), workers, func(i int) {
		p := &positions[i]
		if p.ClosedAt == nil || p.Symbol == "" {
			return
		}
		p.MAE, p.MFE = Excursion(Exposure{
			Symbol: p.Symbol, Side: p.Side, Orders: p.Orders, End: *p.ClosedAt, EndPrice: p.ExitPrice,
			Gross: p.Pnl, Net: p.NetPnl,
		}, fetch)
	})
}

func EnrichOpenMAEMFE(positions []domain.FXOpenPosition, now time.Time, fetch BarFetcher, workers int) {
	if fetch == nil {
		return
	}
	run(len(positions), workers, func(i int) {
		p := &positions[i]
		if p.Symbol == "" || p.CurrentPrice <= 0 {
			return
		}
		var realized float64
		for _, o := range p.Orders {
			realized += o.Trade.Profit
		}
		gross := realized + p.UnrealizedPnl
		p.MAE, p.MFE = Excursion(Exposure{
			Symbol: p.Symbol, Side: p.Side, Orders: p.Orders, End: now, EndPrice: p.CurrentPrice,
			Gross: gross, Net: gross + p.Swap - p.Commission,
		}, fetch)
	})
}

func run(n, workers int, fn func(i int)) {
	if n == 0 {
		return
	}
	if workers <= 0 {
		workers = 2
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				fn(i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

func Excursion(e Exposure, fetch BarFetcher) (mae, mfe *float64) {
	legs := legsOf(e)
	if len(legs) == 0 {
		return nil, nil
	}
	sign := 1.0
	if e.Side == Short {
		sign = -1
	}

	var size, notional, realized float64
	worst, best := math.Inf(1), math.Inf(-1)
	for i, l := range legs {
		if l.opening {
			size += l.quantity
			notional += l.quantity * l.price
		} else if size > 0 {
			avg := notional / size
			realized += sign * (l.price - avg) * l.quantity
			notional -= l.quantity * avg
			size -= l.quantity
		}
		if size <= 0 {
			continue
		}
		segEnd := e.End
		if i+1 < len(legs) {
			segEnd = legs[i+1].at
		}
		high, low, err := extremes(fetch, e.Symbol, l.at, segEnd)
		if err != nil {
			return nil, nil
		}
		if high == nil {
			continue
		}
		avg := notional / size
		adverse, favorable := sign*(*low-avg)*size, sign*(*high-avg)*size
		if e.Side == Short {
			adverse, favorable = favorable, adverse
		}
		worst = math.Min(worst, realized+adverse)
		best = math.Max(best, realized+favorable)
	}
	if size > 0 {
		realized += sign * (e.EndPrice - notional/size) * size
	}

	rate := 1.0
	if realized != 0 && e.Gross != 0 {
		rate = math.Abs(e.Gross / realized)
	}
	costs := e.Net - e.Gross
	var adverse, favorable float64
	if !math.IsInf(worst, 1) {
		adverse = worst * rate
	}
	if !math.IsInf(best, -1) {
		favorable = best * rate
	}
	maeVal := Round8(math.Min(math.Min(adverse+costs, e.Net), 0))
	mfeVal := Round8(math.Max(math.Max(favorable+costs, e.Net), 0))
	return &maeVal, &mfeVal
}

func legsOf(e Exposure) []leg {
	opening := OrderSide(e.Side)
	var legs []leg
	for _, o := range e.Orders {
		t := o.Trade
		if t.Amount <= 0 || t.DoneAt.IsZero() || t.Price <= 0 {
			continue
		}
		legs = append(legs, leg{at: t.DoneAt, quantity: t.Amount, price: t.Price, opening: o.Side == opening})
	}
	sort.SliceStable(legs, func(i, j int) bool {
		if legs[i].at.Equal(legs[j].at) {
			return legs[i].opening && !legs[j].opening
		}
		return legs[i].at.Before(legs[j].at)
	})
	return legs
}

func extremes(fetch BarFetcher, symbol string, from, to time.Time) (high, low *float64, err error) {
	fromMs, toMs := from.UnixMilli(), to.UnixMilli()
	for _, seg := range candlespan.Split(fromMs, toMs) {
		bars, err := fetch(symbol, seg.Interval, time.UnixMilli(seg.StartMs).UTC(), time.UnixMilli(seg.EndMs).UTC())
		if err != nil {
			return nil, nil, err
		}
		barMs := excursion.IntervalMs(seg.Interval)
		for _, b := range bars {
			if !excursion.Inside(b.OpenTime.UnixMilli(), barMs, fromMs, toMs) {
				continue
			}
			if high == nil || b.High > *high {
				h := b.High
				high = &h
			}
			if low == nil || b.Low < *low {
				l := b.Low
				low = &l
			}
		}
	}
	return high, low, nil
}
