package reverse

import "math"

const DefaultEpsilon = 1e-9

type Group[T any] struct {
	Key   string
	Fills []T
}

type Walker[T any] struct {
	epsilon     float64
	seed        func(key string) float64
	position    map[string]float64
	pending     map[string][]T
	opened      map[string][]T
	startedFlat map[string]bool
}

func NewWalker[T any](seed func(key string) float64) *Walker[T] {
	if seed == nil {
		seed = func(string) float64 { return 0 }
	}
	return &Walker[T]{
		epsilon:     DefaultEpsilon,
		seed:        seed,
		position:    make(map[string]float64),
		pending:     make(map[string][]T),
		opened:      make(map[string][]T),
		startedFlat: make(map[string]bool),
	}
}

func SeedFromMap(positions map[string]float64) func(string) float64 {
	return func(key string) float64 { return positions[key] }
}

func (w *Walker[T]) Seed(key string, position float64) {
	if _, seen := w.position[key]; seen {
		return
	}
	if math.Abs(position) < w.epsilon {
		position = 0
	}
	w.position[key] = position
}

func (w *Walker[T]) Push(key string, delta float64, fill T) (Group[T], bool) {
	after, seen := w.position[key]
	if !seen {
		after = w.seed(key)
		if math.Abs(after) < w.epsilon {
			after = 0
		}
		w.position[key] = after
	}

	if len(w.pending[key]) == 0 {
		w.startedFlat[key] = after == 0
	}
	w.pending[key] = append(w.pending[key], fill)

	before := after - delta
	if math.Abs(before) < w.epsilon {
		before = 0
	}
	w.position[key] = before

	if before != 0 {
		return Group[T]{}, false
	}

	fills := w.pending[key]
	w.pending[key] = nil

	for i, j := 0, len(fills)-1; i < j; i, j = i+1, j-1 {
		fills[i], fills[j] = fills[j], fills[i]
	}

	if !w.startedFlat[key] {
		// These fills walked a seeded (still open) position back to zero:
		// they are the fills that opened it.
		w.opened[key] = append(fills, w.opened[key]...)
		return Group[T]{}, false
	}
	return Group[T]{Key: key, Fills: fills}, true
}

func (w *Walker[T]) Flat() bool {
	for _, fills := range w.pending {
		if len(fills) > 0 {
			return false
		}
	}
	return true
}

// Resolved reports whether every seeded position has been walked back to zero
// and no fills are pending: the walk has reached the opening fill of each
// currently open position.
func (w *Walker[T]) Resolved() bool {
	for _, pos := range w.position {
		if pos != 0 {
			return false
		}
	}
	return w.Flat()
}

// Pending returns, oldest first, the fills that belong to the seeded (open)
// positions: those that walked them back to zero plus any still buffered.
// After a resolved walk these are exactly the opening fills of every open
// position.
func (w *Walker[T]) Pending() map[string][]T {
	out := make(map[string][]T, len(w.pending)+len(w.opened))
	for key, fills := range w.opened {
		out[key] = append([]T(nil), fills...)
	}
	for key, fills := range w.pending {
		if len(fills) == 0 {
			continue
		}
		cp := make([]T, len(fills))
		for i, f := range fills {
			cp[len(fills)-1-i] = f
		}
		out[key] = append(out[key], cp...)
	}
	return out
}
