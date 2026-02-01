package reconstructor

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/hyperliquid"
	"sync"
)

func StartPositionBuilders(
	in <-chan []hyperliquid.RawFill,
	out chan<- domain.Position,
	workers int,
) {
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fills := range in {
				out <- BuildPositionFromFills(fills)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()
}
