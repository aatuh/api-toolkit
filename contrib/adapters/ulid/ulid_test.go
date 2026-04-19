package ulid

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestNewULIDGenProducesParseableUniqueIDs(t *testing.T) {
	gen := NewULIDGen()

	first := gen.New()
	second := gen.New()

	if _, err := ulid.Parse(first); err != nil {
		t.Fatalf("Parse(first): %v", err)
	}
	if _, err := ulid.Parse(second); err != nil {
		t.Fatalf("Parse(second): %v", err)
	}
	if first == second {
		t.Fatal("expected generated ULIDs to differ")
	}
}
