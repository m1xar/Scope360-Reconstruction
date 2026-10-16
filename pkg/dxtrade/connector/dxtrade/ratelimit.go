package dxtrade

import (
	"context"
	"sync"
	"time"
)

type limiter struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

func newLimiter(perSecond float64, burst int) *limiter {
	return &limiter{rate: perSecond, burst: float64(burst), tokens: float64(burst), last: time.Now()}
}

func (l *limiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		now := time.Now()
		l.tokens += now.Sub(l.last).Seconds() * l.rate
		if l.tokens > l.burst {
			l.tokens = l.burst
		}
		l.last = now
		if l.tokens >= 1 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}
		wait := time.Duration((1 - l.tokens) / l.rate * float64(time.Second))
		l.mu.Unlock()
		if err := sleepCtx(ctx, wait); err != nil {
			return err
		}
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
