package clock

import "time"

// Clock provides functions to get the current time and the duration of time elapsed since the current time
type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

// RealClock implements Clock based on the actual time
type RealClock struct{}

// Now returns the current time
func (RealClock) Now() time.Time { return time.Now() }

// Since returns the duration of time elapsed since the current time
func (RealClock) Since(t time.Time) time.Duration { return time.Since(t) }
