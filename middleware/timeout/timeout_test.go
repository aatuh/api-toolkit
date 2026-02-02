package timeout

import "testing"

func TestNewRequiresPositiveTimeout(t *testing.T) {
	if _, err := New(Options{Timeout: 0}); err == nil {
		t.Fatal("expected error for zero timeout")
	}
}
