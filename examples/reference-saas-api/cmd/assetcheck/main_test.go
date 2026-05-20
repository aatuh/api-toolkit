package main

import "testing"

func TestAssetCheckAll(t *testing.T) {
	if err := run([]string{"all"}); err != nil {
		t.Fatalf("asset check failed: %v", err)
	}
}

func TestAssetCheckRejectsUnknownMode(t *testing.T) {
	if err := run([]string{"unknown"}); err == nil {
		t.Fatal("expected unknown asset check mode to fail")
	}
}
