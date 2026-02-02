package ports

import "time"

// SystemClock implements Clock using time.Now.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time { return time.Now() }
