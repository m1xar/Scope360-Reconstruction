package clientstate

import (
	"runtime"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

func TestLoadReturnsStoredValue(t *testing.T) {
	var m Map[string]
	client := resty.New()

	m.Store(client, "a")
	m.Store(client, "b")

	got, ok := m.Load(client)
	if !ok || got != "b" {
		t.Fatalf("Load = %q, %v; want the last stored value", got, ok)
	}
	if _, ok := m.Load(resty.New()); ok {
		t.Fatal("Load found a value for another client")
	}
	runtime.KeepAlive(client)
}

func TestEntryGoesAwayWithItsClient(t *testing.T) {
	var m Map[int]
	for i := range 100 {
		m.Store(resty.New(), i)
	}

	deadline := time.Now().Add(5 * time.Second)
	for m.Len() > 0 && time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	if n := m.Len(); n > 0 {
		t.Fatalf("%d entries left after their clients were collected", n)
	}
}
