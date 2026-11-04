package bybit

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	ipWindow     = 5 * time.Second
	ipSoftBudget = 400

	defaultEndpointLimit = 5

	headerLimit       = "X-Bapi-Limit"
	headerLimitStatus = "X-Bapi-Limit-Status"
	headerLimitReset  = "X-Bapi-Limit-Reset-Timestamp"
)

type endpointBucket struct {
	limit     int
	remaining int
	windowEnd time.Time
	resetAt   time.Time
}

// RateLimiter follows Bybit's two limits: a per-IP request budget, and per-endpoint
// limits that Bybit counts per UID, so endpoint buckets are kept per API key.
type RateLimiter struct {
	mu        sync.Mutex
	endpoints map[bucketKey]*endpointBucket
	counts    map[string]int
	ipStamps  []time.Time
	egressIPs int
	requests  int
	throttled int
}

type bucketKey struct {
	apiKey string
	path   string
}

var Limiter = NewRateLimiter()

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		endpoints: make(map[bucketKey]*endpointBucket),
		counts:    make(map[string]int),
	}
}

// SetEgressIPs tells the limiter that requests are spread evenly over n IPs (a
// rotating proxy pool), so the process may spend n times the per-IP budget.
func SetEgressIPs(n int) {
	Limiter.mu.Lock()
	defer Limiter.mu.Unlock()
	Limiter.egressIPs = n
}

// Acquire waits for the IP budget and for the endpoint bucket of apiKey; public
// endpoints pass an empty key.
func (l *RateLimiter) Acquire(apiKey, path string) {
	for {
		l.mu.Lock()
		now := time.Now()
		wait := l.ipWait(now)
		if wait == 0 {
			wait = l.endpointWait(bucketKey{apiKey, path}, now)
		}
		if wait == 0 {
			l.ipStamps = append(l.ipStamps, now)
			l.requests++
			l.mu.Unlock()
			return
		}
		l.mu.Unlock()
		time.Sleep(wait)
	}
}

func (l *RateLimiter) ipWait(now time.Time) time.Duration {
	cutoff := now.Add(-ipWindow)
	i := 0
	for i < len(l.ipStamps) && l.ipStamps[i].Before(cutoff) {
		i++
	}
	l.ipStamps = l.ipStamps[i:]
	if len(l.ipStamps) < ipSoftBudget*max(l.egressIPs, 1) {
		return 0
	}
	return l.ipStamps[0].Add(ipWindow).Sub(now) + 10*time.Millisecond
}

func (l *RateLimiter) endpointWait(key bucketKey, now time.Time) time.Duration {
	l.counts[key.path]++
	if isPublicPath(key.path) {
		return 0
	}
	b, ok := l.endpoints[key]
	if !ok {
		b = &endpointBucket{limit: defaultEndpointLimit, remaining: defaultEndpointLimit}
		l.endpoints[key] = b
	}
	if now.After(b.windowEnd) {
		b.windowEnd = now.Add(time.Second)
		b.remaining = b.limit
	}
	if !b.resetAt.IsZero() && now.Before(b.resetAt) {
		return b.resetAt.Sub(now) + 20*time.Millisecond
	}
	if b.remaining <= 0 {
		return b.windowEnd.Sub(now) + 20*time.Millisecond
	}
	b.remaining--
	return 0
}

func (l *RateLimiter) Observe(apiKey, path string, h http.Header) {
	limit, limitOK := atoi(h.Get(headerLimit))
	status, statusOK := atoi(h.Get(headerLimitStatus))
	if !limitOK && !statusOK {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	key := bucketKey{apiKey, path}
	b, ok := l.endpoints[key]
	if !ok {
		b = &endpointBucket{}
		l.endpoints[key] = b
	}
	if limitOK && limit > 0 {
		b.limit = limit
	}
	if statusOK {
		if status < b.remaining || b.windowEnd.IsZero() {
			b.remaining = status
		}
		if status == 0 {
			l.throttled++
			resetMs, _ := strconv.ParseInt(h.Get(headerLimitReset), 10, 64)
			resetAt := time.UnixMilli(resetMs)
			if resetMs == 0 || !resetAt.After(time.Now()) {
				resetAt = time.Now().Add(200 * time.Millisecond)
			}
			b.resetAt = resetAt
		} else {
			b.resetAt = time.Time{}
		}
	}
}

func (l *RateLimiter) ResetAt(apiKey, path string) time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	if b, ok := l.endpoints[bucketKey{apiKey, path}]; ok && !b.resetAt.IsZero() {
		return b.resetAt
	}
	return time.Now().Add(time.Second)
}

func (l *RateLimiter) Requests() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.requests
}

func (l *RateLimiter) Throttled() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.throttled
}

func (l *RateLimiter) Limits() map[string]int {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]int, len(l.endpoints))
	for key, b := range l.endpoints {
		out[key.path] = max(out[key.path], b.limit)
	}
	return out
}

func (l *RateLimiter) Counts() map[string]int {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]int, len(l.counts))
	for path, n := range l.counts {
		out[path] = n
	}
	return out
}

func atoi(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.Atoi(s)
	return v, err == nil
}
