package pgxpool

import "testing"

func TestNewRejectsInvalidDSN(t *testing.T) {
	if _, err := New("postgres://%zz"); err == nil {
		t.Fatal("expected invalid DSN error")
	}
}
