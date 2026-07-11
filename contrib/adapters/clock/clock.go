package clock

import (
	"time"

	"github.com/aatuh/api-toolkit/v4/ports"
)

// SystemClock implements shared.Clock using time.Now().
type SystemClock struct{}

// Now returns the current UTC time.
func (SystemClock) Now() time.Time { return time.Now().UTC() }

// NewSystemClock creates a new system clock that implements ports.Clock.
func NewSystemClock() ports.Clock {
	return &SystemClock{}
}
