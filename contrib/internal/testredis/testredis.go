// Package testredis provides safe real-Redis fixtures for contrib adapter
// contract tests. It is internal because the endpoint and cleanup semantics are
// test infrastructure rather than an adopter-facing API.
package testredis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// EnvironmentURL names the test-only Redis endpoint setting.
	EnvironmentURL = "API_TOOLKIT_TEST_REDIS_URL"

	testHost = "127.0.0.1"
	testPort = "56379"
	testDB   = 15
)

// Harness owns an isolated Redis key prefix. Close deletes only keys matching
// that prefix and then closes the client.
type Harness struct {
	Client *redis.Client
	Prefix string

	url string
}

// New opens a real Redis harness and registers cleanup with t. It fails the
// test when the dedicated local test endpoint is unavailable.
func New(t testing.TB) *Harness {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	h, err := Open(ctx)
	if err != nil {
		t.Fatalf("open real Redis test harness: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := h.Close(cleanupCtx); err != nil {
			t.Errorf("clean up real Redis test harness: %v", err)
		}
	})
	return h
}

// Open creates a key-isolated harness from EnvironmentURL. The configured URL
// must use the dedicated loopback endpoint without credentials.
func Open(ctx context.Context) (*Harness, error) {
	endpoint, err := testURLFromEnvironment()
	if err != nil {
		return nil, err
	}
	options, err := redis.ParseURL(endpoint)
	if err != nil {
		return nil, errors.New("parse real Redis test endpoint")
	}
	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, errors.New("connect to real Redis test endpoint")
	}
	prefix, err := randomPrefix()
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Harness{Client: client, Prefix: prefix, url: endpoint}, nil
}

// URL returns the dedicated credential-free Redis test endpoint for clients
// that need independent connection pools. It is test-only infrastructure.
func (h *Harness) URL() string {
	if h == nil {
		return ""
	}
	return h.url
}

// Key prefixes a fixture key so concurrent tests cannot share state.
func (h *Harness) Key(part string) string {
	if h == nil {
		return part
	}
	return h.Prefix + part
}

// CanceledContext returns an already-canceled context for cancellation tests.
func (h *Harness) CanceledContext(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// NewClient opens an independent Redis client against the same dedicated test
// endpoint. Callers own the returned client and must close it.
func (h *Harness) NewClient() (*redis.Client, error) {
	if h == nil || h.url == "" {
		return nil, errors.New("Redis test harness is not initialized")
	}
	options, err := redis.ParseURL(h.url)
	if err != nil {
		return nil, errors.New("parse real Redis test endpoint")
	}
	return redis.NewClient(options), nil
}

// InterruptClient kills the target client's current Redis connection. A
// subsequent command should use the go-redis reconnect behavior.
func (h *Harness) InterruptClient(ctx context.Context, target *redis.Client) error {
	if h == nil || h.Client == nil || target == nil {
		return errors.New("Redis test client is required")
	}
	id, err := target.ClientID(ctx).Result()
	if err != nil {
		return errors.New("identify Redis test connection")
	}
	killed, err := h.Client.ClientKillByFilter(ctx, "ID", strconv.FormatInt(id, 10)).Result()
	if err != nil {
		return errors.New("interrupt Redis test connection")
	}
	if killed != 1 {
		return fmt.Errorf("interrupt Redis test connection: killed %d clients", killed)
	}
	return nil
}

// Close deletes this harness's key prefix and closes the Redis client. It is
// safe to call once; it never flushes a Redis database.
func (h *Harness) Close(ctx context.Context) error {
	if h == nil || h.Client == nil {
		return nil
	}
	defer func() { _ = h.Client.Close() }()
	var cursor uint64
	for {
		keys, next, err := h.Client.Scan(ctx, cursor, h.Prefix+"*", 100).Result()
		if err != nil {
			return errors.New("scan Redis test keys")
		}
		if len(keys) > 0 {
			if err := h.Client.Unlink(ctx, keys...).Err(); err != nil {
				return errors.New("delete Redis test keys")
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func testURLFromEnvironment() (string, error) {
	endpoint := strings.TrimSpace(os.Getenv(EnvironmentURL))
	if endpoint == "" {
		endpoint = "redis://" + testHost + ":" + testPort + "/" + strconv.Itoa(testDB)
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", errors.New("parse Redis test endpoint")
	}
	if parsed.Scheme != "redis" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("Redis test endpoint must be a credential-free redis URL")
	}
	host, port, err := net.SplitHostPort(parsed.Host)
	if err != nil || !isLoopback(host) || port != testPort {
		return "", fmt.Errorf("Redis test endpoint must use loopback port %s", testPort)
	}
	if parsed.Path != "/"+strconv.Itoa(testDB) {
		return "", fmt.Errorf("Redis test endpoint must use database %d", testDB)
	}
	return endpoint, nil
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func randomPrefix() (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.New("generate Redis test key prefix")
	}
	return "api-toolkit-test:" + hex.EncodeToString(bytes) + ":", nil
}
