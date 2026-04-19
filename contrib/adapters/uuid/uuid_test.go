package uuid

import (
	"testing"

	googleuuid "github.com/google/uuid"
)

func TestNewUUIDGenProducesParseableUniqueIDs(t *testing.T) {
	gen := NewUUIDGen()

	first := gen.New()
	second := gen.New()

	if _, err := googleuuid.Parse(first); err != nil {
		t.Fatalf("Parse(first): %v", err)
	}
	if _, err := googleuuid.Parse(second); err != nil {
		t.Fatalf("Parse(second): %v", err)
	}
	if first == second {
		t.Fatal("expected generated UUIDs to differ")
	}
}
