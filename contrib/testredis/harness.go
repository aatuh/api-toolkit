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
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// EnableEnv must be set to "1" before the harness will use Redis.
	EnableEnv = "API_TOOLKIT_TEST_REDIS"
	// URLEnv contains the explicit test-only Redis URL.
	URLEnv = "API_TOOLKIT_TEST_REDIS_URL"

	testDatabase     = 15
	operationTimeout = 10 * time.Second
)

type config struct {
	endpoint string
	addr     string
	database int
}

// Harness owns one cryptographically random key prefix in the dedicated test
// Redis database. Cleanup deletes only that prefix and never flushes a database.
type Harness struct {
	client      *redis.Client
	config      config
	prefix      string
	serverMajor int

	closeMu sync.Mutex
	closed  bool
}

// New creates an isolated Redis key prefix for t. It fails the test when Redis
// is unavailable or the configured URL is not a dedicated local/service test
// endpoint.
func New(t testing.TB) *Harness {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	h, err := Open(ctx)
	if err != nil {
		t.Fatalf("open real Redis test harness: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), operationTimeout)
		defer cleanupCancel()
		if err := h.Close(cleanupCtx); err != nil {
			t.Errorf("clean up real Redis test harness: %v", err)
		}
	})
	return h
}

// Open connects to the explicitly enabled test-only Redis endpoint. The caller
// owns the returned harness and must close it.
func Open(ctx context.Context) (*Harness, error) {
	cfg, err := configFromEnv()
	if err != nil {
		return nil, err
	}
	client, err := clientForDatabase(cfg, cfg.database)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, sanitizedRedisError(err)
	}
	major, err := serverMajor(ctx, client)
	if err != nil {
		_ = client.Close()
		return nil, sanitizedRedisError(err)
	}
	prefix, err := randomPrefix()
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Harness{client: client, config: cfg, prefix: prefix, serverMajor: major}, nil
}

// Client returns the harness client in the dedicated test database.
func (h *Harness) Client() *redis.Client {
	if h == nil {
		return nil
	}
	return h.client
}

// URL returns the credential-free, test-only endpoint. Do not log this value.
func (h *Harness) URL() string {
	if h == nil {
		return ""
	}
	return h.config.endpoint
}

// Addr returns the credential-free host:port used by generated service paths.
func (h *Harness) Addr() string {
	if h == nil {
		return ""
	}
	return h.config.addr
}

// ServerMajorVersion returns the connected Redis major version.
func (h *Harness) ServerMajorVersion() int {
	if h == nil {
		return 0
	}
	return h.serverMajor
}

// Key prefixes part so concurrent tests cannot share state.
func (h *Harness) Key(part string) string {
	if h == nil {
		return part
	}
	return h.prefix + part
}

// NewClient opens an independent client against the dedicated test database.
// Callers own the returned client and must close it.
func (h *Harness) NewClient() (*redis.Client, error) {
	if h == nil || h.config.endpoint == "" {
		return nil, errors.New("redis test harness is not initialized")
	}
	return clientForDatabase(h.config, h.config.database)
}

// NewClientForDatabase opens an independent client for a generated service
// path that fixes its own Redis database. Only databases 0 through 15 are
// available on the dedicated test service.
func (h *Harness) NewClientForDatabase(database int) (*redis.Client, error) {
	if h == nil || h.config.endpoint == "" {
		return nil, errors.New("redis test harness is not initialized")
	}
	if database < 0 || database > testDatabase {
		return nil, errors.New("redis test database must be between 0 and 15")
	}
	return clientForDatabase(h.config, database)
}

// InterruptClient terminates one connection owned by target. A subsequent
// command exercises the go-redis reconnect path.
func (h *Harness) InterruptClient(ctx context.Context, target *redis.Client) error {
	if h == nil || h.client == nil || target == nil {
		return errors.New("redis test client is required")
	}
	id, err := target.ClientID(ctx).Result()
	if err != nil {
		return sanitizedRedisError(err)
	}
	killed, err := h.client.ClientKillByFilter(ctx, "ID", strconv.FormatInt(id, 10)).Result()
	if err != nil {
		return sanitizedRedisError(err)
	}
	if killed != 1 {
		return fmt.Errorf("interrupt Redis test connection: killed %d clients", killed)
	}
	return nil
}

// Close deletes this harness's key prefix and closes its client. It is
// idempotent and never flushes the Redis database.
func (h *Harness) Close(ctx context.Context) error {
	if h == nil {
		return nil
	}
	h.closeMu.Lock()
	defer h.closeMu.Unlock()
	if h.closed {
		return nil
	}
	h.closed = true
	if h.client == nil {
		return nil
	}
	defer func() { _ = h.client.Close() }()
	var cursor uint64
	for {
		keys, next, err := h.client.Scan(ctx, cursor, h.prefix+"*", 100).Result()
		if err != nil {
			return sanitizedRedisError(err)
		}
		if len(keys) > 0 {
			if err := h.client.Unlink(ctx, keys...).Err(); err != nil {
				return sanitizedRedisError(err)
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func configFromEnv() (config, error) {
	if os.Getenv(EnableEnv) != "1" {
		return config{}, fmt.Errorf("%s must be set to 1", EnableEnv)
	}
	return parseConfig(os.Getenv(URLEnv))
}

func parseConfig(endpoint string) (config, error) {
	if strings.TrimSpace(endpoint) == "" {
		return config{}, fmt.Errorf("%s is required", URLEnv)
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return config{}, errors.New("test Redis URL is malformed")
	}
	if parsed.Scheme != "redis" || parsed.Opaque != "" {
		return config{}, errors.New("test Redis URL must use redis scheme")
	}
	if parsed.User != nil {
		return config{}, errors.New("test Redis URL must not contain credentials")
	}
	if !isTestHost(parsed.Hostname()) {
		return config{}, errors.New("test Redis URL host must be localhost, loopback, or redis")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return config{}, errors.New("test Redis URL must contain a valid explicit port")
	}
	if parsed.EscapedPath() != "/"+strconv.Itoa(testDatabase) {
		return config{}, fmt.Errorf("test Redis URL must target database %d", testDatabase)
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return config{}, errors.New("test Redis URL must not contain query parameters or a fragment")
	}
	return config{endpoint: endpoint, addr: parsed.Host, database: testDatabase}, nil
}

func isTestHost(host string) bool {
	if host == "localhost" || host == "redis" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func clientForDatabase(cfg config, database int) (*redis.Client, error) {
	options, err := redis.ParseURL(cfg.endpoint)
	if err != nil {
		return nil, errors.New("configure Redis test client")
	}
	options.DB = database
	options.MaxRetries = -1
	return redis.NewClient(options), nil
}

func serverMajor(ctx context.Context, client *redis.Client) (int, error) {
	info, err := client.Info(ctx, "server").Result()
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(info, "\n") {
		if !strings.HasPrefix(line, "redis_version:") {
			continue
		}
		version := strings.TrimSpace(strings.TrimPrefix(line, "redis_version:"))
		majorText, _, _ := strings.Cut(version, ".")
		major, err := strconv.Atoi(majorText)
		if err == nil && major > 0 {
			return major, nil
		}
		break
	}
	return 0, errors.New("redis test service reported an invalid version")
}

func randomPrefix() (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.New("generate Redis test key prefix")
	}
	return "api-toolkit-test:" + hex.EncodeToString(bytes) + ":", nil
}

func sanitizedRedisError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New("redis test service operation failed")
}
