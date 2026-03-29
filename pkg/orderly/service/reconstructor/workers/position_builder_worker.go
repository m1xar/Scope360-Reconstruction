package workers

import (
	"sync"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/builders"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/envelope"
)

func StartPositionBuilders(
	in <-chan envelope.TradeEnvelope,
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
