package binance

import (
	"strconv"
	"testing"
	"time"
)

func TestBudgetScalesWithEgressIPs(t *testing.T) {
	l := &WeightLimiter{egressIPs: 4}

	done := make(chan struct{})
	go func() {
		l.Acquire(weightSoftBudget * 3)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("four IPs waited for three IPs' worth of weight")
	}
}

func TestUsedWeightHeaderIsIgnoredWithSeveralIPs(t *testing.T) {
	l := &WeightLimiter{egressIPs: 2, minute: time.Now().Unix() / 60}

	l.Observe(strconv.Itoa(weightSoftBudget))
	if got := l.Used(); got != 0 {
		t.Fatalf("used = %d, want the header of one IP ignored", got)
	}

	l.egressIPs = 1
	l.Observe(strconv.Itoa(weightSoftBudget))
	if got := l.Used(); got != weightSoftBudget {
		t.Fatalf("used = %d with one IP, want the header applied", got)
	}
}
