package redis

import (
	"context"
	"errors"
	"testing"
)

func TestParseRedisAddrsRejectsEmptyAndSplits(t *testing.T) {
	if got := parseRedisAddrs(" "); len(got) != 0 {
		t.Fatalf("empty parseRedisAddrs() = %#v", got)
	}
	got := parseRedisAddrs("localhost:6379, redis-2:6379 ;redis-3:6379")
	if len(got) != 3 || got[0] != "localhost:6379" || got[2] != "redis-3:6379" {
		t.Fatalf("parseRedisAddrs() = %#v", got)
	}
}

func TestCacheCheckRequiresClient(t *testing.T) {
	if err := (*Cache)(nil).Check(context.Background()); !errors.Is(err, ErrRedisClientMissing) {
		t.Fatalf("nil Check() error = %v, want %v", err, ErrRedisClientMissing)
	}
	if err := (&Cache{}).Check(context.Background()); !errors.Is(err, ErrRedisClientMissing) {
		t.Fatalf("missing client Check() error = %v, want %v", err, ErrRedisClientMissing)
	}
}
