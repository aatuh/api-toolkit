package httpclient

import (
	"context"
	"testing"
)

func TestSemaphoreBulkheadAcquire(t *testing.T) {
	bulkhead, err := NewSemaphoreBulkhead(1)
	if err != nil {
		t.Fatalf("new bulkhead: %v", err)
	}
	release, err := bulkhead.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := bulkhead.Acquire(ctx); err == nil {
		t.Fatal("expected acquire to fail with canceled context")
	}
	release()
}
