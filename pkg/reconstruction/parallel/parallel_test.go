package parallel

import (
	"errors"
	"testing"
)

func TestRunWaitsForAllAndReturnsFirstError(t *testing.T) {
	var a, b int
	errB := errors.New("b failed")
	err := Run(
		func() error { a = 1; return nil },
		nil,
		func() error { b = 2; return errB },
	)
	if !errors.Is(err, errB) {
		t.Fatalf("err = %v, want %v", err, errB)
	}
	if a != 1 || b != 2 {
		t.Fatalf("results a=%d b=%d, want both set", a, b)
	}
	if err := Run(); err != nil {
		t.Fatalf("empty Run: %v", err)
	}
}
