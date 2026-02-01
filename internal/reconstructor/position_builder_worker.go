package reconstructor

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"sync"
)

func StartPositionBuilders(
	in <-chan TradeEnvelope,
	out chan<- domain.Position,
	workers int,
) {
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for env := range in {
				out <- BuildPositionFromEnvelope(env)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()
}
