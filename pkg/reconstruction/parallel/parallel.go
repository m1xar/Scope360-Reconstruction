// Package parallel runs independent steps concurrently and reports the first
// failure, which is all the reconstruction loaders need.
package parallel

import "sync"

// Run executes every fn concurrently, waits for all of them and returns the
// first error. Callers write their results into closure variables; a
// non-nil error leaves them unspecified.
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
