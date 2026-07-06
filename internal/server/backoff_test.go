package server //nolint:testpackage // white-box test: exercises the unexported backoffDelay overflow guard.

import (
	"testing"
	"time"
)

func TestBackoffDelayCapsAndAvoidsOverflow(t *testing.T) {
	s := &Server{cfg: Config{RetryBaseDelay: 250 * time.Millisecond}}

	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 250 * time.Millisecond},
		{1, 500 * time.Millisecond},
		{2, time.Second},
		{7, maxRetryBackoff},   // 250ms<<7 = 32s, over the ceiling
		{40, maxRetryBackoff},  // would overflow int64
		{100, maxRetryBackoff}, // shift past the type width wraps to 0
	}
	for _, c := range cases {
		if got := s.backoffDelay(c.attempt); got != c.want {
			t.Errorf("backoffDelay(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}

	// A non-positive base yields no delay.
	if got := (&Server{cfg: Config{RetryBaseDelay: 0}}).backoffDelay(3); got != 0 {
		t.Errorf("backoffDelay with zero base = %v, want 0", got)
	}

	// No attempt may ever produce a negative or above-cap delay.
	for attempt := range 128 {
		if got := s.backoffDelay(attempt); got < 0 || got > maxRetryBackoff {
			t.Fatalf("backoffDelay(%d) = %v out of range", attempt, got)
		}
	}
}
