// Package clientstate attaches state to a resty client without keeping it alive.
//
// Callers build a client per sync, so a plain map keyed by the client pointer holds
// every client (and its transport and idle connections) for the life of the process.
package clientstate

import (
	"runtime"
	"sync"
	"weak"

	"github.com/go-resty/resty/v2"
)

// Map holds one value per live client; an entry goes away once its client is collected.
type Map[V any] struct {
	m sync.Map // weak.Pointer[resty.Client] -> V
}

func (m *Map[V]) Load(client *resty.Client) (V, bool) {
	v, ok := m.m.Load(weak.Make(client))
	if !ok {
		var zero V
		return zero, false
	}
	return v.(V), true
}

func (m *Map[V]) Store(client *resty.Client, v V) {
	key := weak.Make(client)
	if _, loaded := m.m.Swap(key, v); !loaded {
		runtime.AddCleanup(client, func(key weak.Pointer[resty.Client]) { m.m.Delete(key) }, key)
	}
}

// Len counts the live entries.
func (m *Map[V]) Len() int {
	n := 0
	m.m.Range(func(any, any) bool { n++; return true })
	return n
}
