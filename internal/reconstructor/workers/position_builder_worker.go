package workers

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/reconstructor/builders"
	"net/http"
	"sync"
)

func StartPositionBuilders(
	in <-chan domain.TradeEnvelope,
	out chan<- domain.Position,
	workers int,
	client *http.Client,
) {
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for env := range in {
				out <- builders.BuildPositionFromEnvelope(env)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()
}
