package devheaders

import "testing"

func TestNewRequiresUserHeaderWhenEnabled(t *testing.T) {
	if _, err := New(Config{Enabled: true}, nil); err == nil {
		t.Fatal("expected error for missing user header")
	}
}
