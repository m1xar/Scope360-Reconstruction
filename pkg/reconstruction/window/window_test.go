package window

import "testing"

func TestBackward(t *testing.T) {
	spans := Backward(100, 0, 40)
	want := []Span{{61, 100}, {21, 60}, {1, 20}}
	if len(spans) != len(want) {
		t.Fatalf("got %v", spans)
	}
	for i := range want {
		if spans[i] != want[i] {
			t.Fatalf("span %d = %v, want %v", i, spans[i], want[i])
		}
	}
	if Backward(10, 10, 5) != nil || Backward(10, 0, 0) != nil {
		t.Fatal("expected nil for empty ranges")
	}
}
