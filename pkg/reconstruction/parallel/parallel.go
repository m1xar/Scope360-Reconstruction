package parallel

import "sync"

func Run(fns ...func() error) error {
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		first error
	)
	for _, fn := range fns {
		if fn == nil {
			continue
		}
		wg.Add(1)
		go func(fn func() error) {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				if first == nil {
					first = err
				}
				mu.Unlock()
			}
		}(fn)
	}
	wg.Wait()
	return first
}
