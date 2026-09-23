package symbols

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
)

const refreshInterval = 5 * time.Minute

type Entry struct {
	Symbol string `json:"symbol"`
	Pair   string `json:"pair"`
}

type Registry struct {
	mu          sync.Mutex
	byKey       map[string]string
	symbols     map[string]struct{}
	seed        []Entry
	refreshedAt time.Time
}

func Key(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func New(seedJSON []byte) *Registry {
	var seed []Entry
	if len(seedJSON) > 0 {
		_ = json.Unmarshal(seedJSON, &seed)
	}
	r := &Registry{seed: seed}
	r.rebuild(nil)
	return r
}

func (r *Registry) Resolve(input string, fetch func() ([]Entry, error)) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("symbol is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if symbol, ok := r.lookup(input); ok {
		return symbol, nil
	}
	if fetch == nil || time.Since(r.refreshedAt) < refreshInterval {
		return "", fmt.Errorf("symbol %q not found", input)
	}

	fresh, err := fetch()
	if err != nil {
		return "", err
	}
	r.rebuild(fresh)
	r.refreshedAt = time.Now()

	if symbol, ok := r.lookup(input); ok {
		return symbol, nil
	}
	return "", fmt.Errorf("symbol %q not found", input)
}

func (r *Registry) lookup(input string) (string, bool) {
	if _, ok := r.symbols[input]; ok {
		return input, true
	}
	symbol, ok := r.byKey[Key(input)]
	return symbol, ok
}

func (r *Registry) rebuild(fresh []Entry) {
	r.byKey = make(map[string]string, len(fresh)+len(r.seed))
	r.symbols = make(map[string]struct{}, len(fresh)+len(r.seed))
	for _, e := range fresh {
		r.add(e)
	}
	for _, e := range r.seed {
		r.add(e)
	}
}

func (r *Registry) add(e Entry) {
	if e.Symbol == "" {
		return
	}
	r.symbols[e.Symbol] = struct{}{}
	key := Key(e.Pair)
	if key == "" {
		return
	}
	if _, ok := r.byKey[key]; !ok {
		r.byKey[key] = e.Symbol
	}
}
