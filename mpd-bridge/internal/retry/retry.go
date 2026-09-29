// Package retry provides the reconnect backoff shared by the MPD and Snapcast clients.
package retry

import (
	"context"
	"math/rand/v2"
	"time"
)

// Backoff yields exponential delays from 0.5 s doubling up to 30 s, with ±20 % jitter.
type Backoff struct{ n int }

func (b *Backoff) Next() time.Duration {
	d := min(500*time.Millisecond<<min(b.n, 6), 30*time.Second)
	b.n++
	return time.Duration(float64(d) * (0.8 + 0.4*rand.Float64()))
}

func (b *Backoff) Reset() { b.n = 0 }

// Sleep waits for d and reports false if ctx ended first.
func Sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
