package enrich

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterSpacesCalls(t *testing.T) {
	rl := newRateLimiter(30 * time.Millisecond)
	ctx := context.Background()

	start := time.Now()
	for range 3 {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Wait: %v", err)
		}
	}
	elapsed := time.Since(start)
	if elapsed < 60*time.Millisecond {
		t.Fatalf("3 calls at 30ms spacing took %s, expected >= 60ms", elapsed)
	}
}

func TestRateLimiterRespectsContextCancellation(t *testing.T) {
	rl := newRateLimiter(time.Hour)
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("first Wait: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := rl.Wait(ctx); err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
}
