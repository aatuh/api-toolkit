//go:build redis

package cliintegration

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v4/internal/testredis"
)

func TestGeneratedFullProfileUsesRealRedis(t *testing.T) {
	h := testredis.New(t)
	endpoint, err := url.Parse(h.URL())
	if err != nil {
		t.Fatalf("parse Redis test endpoint: %v", err)
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	serviceDir := filepath.Join(t.TempDir(), "service")
	output, err := runCLI(context.Background(), append([]string{
		"new", "service",
		"--module", "example.com/redis-integration",
		"--profile", "saas-api-full",
		"--auth", "api-key",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	})...)
	if err != nil {
		t.Fatalf("generate full profile: %v\n%s", err, output)
	}
	tidy := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	tidy.Dir = serviceDir
	tidy.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	if output, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("tidy generated profile: %v\n%s", err, output)
	}
	key := h.Key("generated-cache")
	t.Cleanup(func() { _ = h.Client.Del(context.Background(), "api:"+key).Err() })
	testSource := fmt.Sprintf(`package redis

import (
	"context"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v4/middleware/idempotency"
)

func TestGeneratedRealRedisPath(t *testing.T) {
	ctx := context.Background()
	cache, err := OpenCache(ctx, %q)
	if err != nil { t.Fatalf("OpenCache: %%v", err) }
	defer func() { _ = cache.Close() }()
	if err := cache.Store.Set(ctx, %q, []byte("value"), time.Second); err != nil { t.Fatalf("cache set: %%v", err) }
	value, found, err := cache.Store.Get(ctx, %q)
	if err != nil || !found || string(value) != "value" { t.Fatalf("cache get = (%%q, %%t, %%v)", value, found, err) }
	if err := cache.Store.Delete(ctx, %q); err != nil { t.Fatalf("cache delete: %%v", err) }
	idem, err := OpenIdempotencyStore(ctx, %q, %q)
	if err != nil { t.Fatalf("OpenIdempotencyStore: %%v", err) }
	defer func() { _ = idem.Close() }()
	if ok, err := idem.Store.TryBegin(ctx, "request", idempotency.Record{State: idempotency.StateInFlight, ReservationToken: "token"}, time.Second); err != nil || !ok { t.Fatalf("TryBegin = (%%t, %%v)", ok, err) }
	if err := idem.Store.ReleaseReservation(ctx, "request", "token"); err != nil { t.Fatalf("ReleaseReservation: %%v", err) }
	rate, err := OpenRateLimiter(ctx, %q, %q, 1, 1)
	if err != nil { t.Fatalf("OpenRateLimiter: %%v", err) }
	defer func() { _ = rate.Close() }()
	if allowed, _, err := rate.Limiter.Allow(ctx, "tenant-a"); err != nil || !allowed { t.Fatalf("Allow = (%%t, %%v)", allowed, err) }
}
	`, endpoint.Host, key, key, key, endpoint.Host, h.Key("generated-idempotency:"), endpoint.Host, h.Key("generated-ratelimit:"))
	path := filepath.Join(serviceDir, "internal", "adapters", "redis", "real_redis_test.go")
	if err := os.WriteFile(path, []byte(testSource), 0o600); err != nil {
		t.Fatalf("write generated Redis contract: %v", err)
	}
	contract := exec.CommandContext(context.Background(), "go", "test", "./internal/adapters/redis", "-run", "TestGeneratedRealRedisPath", "-count=1")
	contract.Dir = serviceDir
	contract.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	if output, err := contract.CombinedOutput(); err != nil {
		t.Fatalf("run generated Redis contract: %v\n%s", err, output)
	}
}
