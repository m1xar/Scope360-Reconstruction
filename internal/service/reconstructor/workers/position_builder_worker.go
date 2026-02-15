package workers

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/builders"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/models"
	"sync"
)

func StartPositionBuilders(
	in <-chan models.TradeEnvelope,
	out chan<- domain.Position,
	workers int,
) {
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for env := range in {
				pos, err := builders.BuildPositionFromEnvelope(env)
				if err != nil {
					continue
				}
				out <- pos
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()
}
