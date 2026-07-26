package enrich

import (
	"context"
	"sync"
	"time"
)

// rateLimiter enforces a minimum gap between successive Wait calls — the
// simplest thing that satisfies MusicBrainz's ~1 req/sec usage policy
// without pulling in a token-bucket dependency (golang.org/x/time/rate)
// for a need this small.
type rateLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

func newRateLimiter(interval time.Duration) *rateLimiter {
	return &rateLimiter{interval: interval}
}

// Wait blocks until interval has elapsed since the previous call's Wait
// returned, or ctx is cancelled first.
func (r *rateLimiter) Wait(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.last.IsZero() {
		if wait := r.interval - time.Since(r.last); wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	r.last = time.Now()
	return nil
}
