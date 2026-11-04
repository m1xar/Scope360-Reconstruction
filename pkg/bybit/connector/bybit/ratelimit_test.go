package bybit

import (
	"net/http"
	"testing"
	"time"
)

const privatePath = "/v5/execution/list"

func TestEndpointBucketsAreKeptPerAPIKey(t *testing.T) {
	l := NewRateLimiter()

	for range defaultEndpointLimit {
		if wait := l.endpointWait(bucketKey{"a", privatePath}, time.Now()); wait != 0 {
			t.Fatalf("key a waited %v within its limit", wait)
		}
	}
	if wait := l.endpointWait(bucketKey{"a", privatePath}, time.Now()); wait == 0 {
		t.Fatal("key a passed its endpoint limit")
	}
	if wait := l.endpointWait(bucketKey{"b", privatePath}, time.Now()); wait != 0 {
		t.Fatalf("key b waited %v because key a used its limit", wait)
	}
}

func TestObservedLimitAppliesOnlyToItsKey(t *testing.T) {
	l := NewRateLimiter()
	h := http.Header{}
	h.Set(headerLimit, "50")
	h.Set(headerLimitStatus, "0")
	h.Set(headerLimitReset, "")

	l.Observe("a", privatePath, h)

	if wait := l.endpointWait(bucketKey{"a", privatePath}, time.Now()); wait == 0 {
		t.Fatal("key a ignored its exhausted bucket")
	}
	if wait := l.endpointWait(bucketKey{"b", privatePath}, time.Now()); wait != 0 {
		t.Fatalf("key b waited %v on key a's exhausted bucket", wait)
	}
}

func TestIPBudgetScalesWithEgressIPs(t *testing.T) {
	l := NewRateLimiter()
	now := time.Now()
	for range ipSoftBudget {
		l.ipStamps = append(l.ipStamps, now)
	}
	if wait := l.ipWait(now); wait == 0 {
		t.Fatal("one IP passed its budget")
	}

	l.egressIPs = 3
	if wait := l.ipWait(now); wait != 0 {
		t.Fatalf("three IPs waited %v after one IP's budget", wait)
	}
}
