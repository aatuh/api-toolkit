package clock

import (
	"time"

	"github.com/aatuh/api-toolkit/ports"
)

// SystemClock implements shared.Clock using time.Now().
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// NewSystemClock creates a new system clock that implements ports.Clock.
func NewSystemClock() ports.Clock {
	return &SystemClock{}
}
