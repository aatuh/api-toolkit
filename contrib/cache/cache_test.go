package cache

import (
	"errors"
	"testing"
)

func TestValidateKey(t *testing.T) {
	t.Parallel()

	if err := ValidateKey("tenant:widget:123"); err != nil {
		t.Fatalf("ValidateKey() error = %v", err)
	}
	if err := ValidateKey(" "); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("ValidateKey() error = %v, want ErrInvalidKey", err)
	}
}

func TestCloneBytes(t *testing.T) {
	t.Parallel()

	input := []byte("cached")
	cloned := CloneBytes(input)
	cloned[0] = 'C'
	if string(input) != "cached" {
		t.Fatalf("CloneBytes mutated input: %q", input)
	}
	if CloneBytes(nil) != nil {
		t.Fatal("CloneBytes(nil) should return nil")
	}
}
