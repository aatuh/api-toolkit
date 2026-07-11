package clock

import (
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v4/ports"
)

var _ ports.Clock = (*SystemClock)(nil)

func TestSystemClockImplementsClockAndReturnsUTC(t *testing.T) {
	clock := NewSystemClock()

	before := time.Now().UTC().Add(-time.Second)
	got := clock.Now()
	after := time.Now().UTC().Add(time.Second)

	if got.Location() != time.UTC {
		t.Fatalf("Now location = %v, want UTC", got.Location())
	}
	if got.Before(before) || got.After(after) {
		t.Fatalf("Now = %v, want between %v and %v", got, before, after)
	}
}
