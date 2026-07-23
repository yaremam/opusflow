// Package duration computes an audio file's playback length directly from
// its container/frame headers, without decoding audio samples.
package duration

import (
	"time"
)

// secondsToDuration converts a sample-count/rate ratio into a time.Duration.
func secondsToDuration(samples, rate float64) time.Duration {
	return time.Duration(samples / rate * float64(time.Second))
}
