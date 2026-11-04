package binance

import (
	"strconv"
	"sync"
	"time"
)

const (
	weightLimitPerMinute = 2400
	weightSoftBudget     = 2000
)

// WeightLimiter keeps the process under Binance's request weight, which is counted
// per IP. By default it assumes every request leaves from one IP.
type WeightLimiter struct {
	mu        sync.Mutex
	minute    int64
	used      int
	egressIPs int
}

var Limiter = &WeightLimiter{}

// SetEgressIPs tells the limiter that requests are spread evenly over n IPs (a
// rotating proxy pool), so the process may spend n times the per-IP budget.
// With more than one IP the used-weight header describes only the IP that answered,
// so it no longer feeds the shared count; 429 backoff still applies per request.
func SetEgressIPs(n int) {
	Limiter.mu.Lock()
	defer Limiter.mu.Unlock()
	Limiter.egressIPs = n
}

func (l *WeightLimiter) budgetLocked() int {
	return weightSoftBudget * max(l.egressIPs, 1)
}

func (l *WeightLimiter) Acquire(weight int) {
	if weight <= 0 {
		weight = 1
	}
	for {
		l.mu.Lock()
		now := time.Now()
		minute := now.Unix() / 60
		if minute != l.minute {
			l.minute = minute
			l.used = 0
		}
		if l.used+weight <= l.budgetLocked() {
			l.used += weight
			l.mu.Unlock()
			return
		}
		l.mu.Unlock()

		nextMinute := time.Unix((minute+1)*60, 0)
		time.Sleep(time.Until(nextMinute) + 100*time.Millisecond)
	}
}

func (l *WeightLimiter) Observe(header string) {
	if header == "" {
		return
	}
	used, err := strconv.Atoi(header)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.egressIPs > 1 {
		return
	}
	minute := time.Now().Unix() / 60
	if minute != l.minute {
		l.minute = minute
		l.used = 0
	}
	if used > l.used {
		l.used = used
	}
}

func (l *WeightLimiter) Used() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.used
}
