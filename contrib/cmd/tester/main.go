package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/envvar"
)

var env = envvar.New()

// Config holds all configuration for the test runner
type Config struct {
	// API configuration
	APIHost        string
	SkipAPIWait    bool
	APIWaitTimeout time.Duration

	// Test configuration
	PackagePattern string
	TestPattern    string
	Flags          string
	FastMode       bool

	// Cache configuration
	CacheEnabled bool
	CacheTTL     int
	CacheDir     string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		APIHost:        env.GetOr("API_HOST", ""),
		SkipAPIWait:    env.GetBoolOr("SKIP_API_WAIT", false),
		APIWaitTimeout: env.GetDurationOr("API_WAIT_TIMEOUT", 20*time.Second),
		PackagePattern: env.GetOr("PKG", "./..."),
		TestPattern:    env.GetOr("TEST_PATTERN", ""),
		Flags:          env.GetOr("FLAGS", ""),
		FastMode:       env.GetBoolOr("FAST", false),
		CacheEnabled:   env.GetBoolOr("TEST_CACHE_ENABLED", true),
		CacheTTL:       env.GetIntOr("TEST_CACHE_TTL", 3600),
		CacheDir:       env.GetOr("TEST_CACHE_DIR", ".test-cache"),
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func showHelp() {
	fmt.Println("This tool runs Go tests with caching, coverage, and API health checking.")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println()
	fmt.Println("  API_HOST              API server URL (required unless SKIP_API_WAIT is set)")
	fmt.Println("  SKIP_API_WAIT         Skip waiting for API server (any non-empty value)")
	fmt.Println("  API_WAIT_TIMEOUT      Max time to wait for API readiness (default: 20s, <=0 waits until context cancel)")
	fmt.Println("  PKG                   Package pattern to test (default: ./...)")
	fmt.Println("  TEST_PATTERN          Test name pattern to run (e.g., TestFoo)")
	fmt.Println("  FLAGS                 Additional go test flags (e.g., -race -count=1)")
	fmt.Println("  FAST                  Enable fast mode (any non-empty value)")
	fmt.Println("  TEST_CACHE_ENABLED    Enable test result caching (default: true)")
	fmt.Println("  TEST_CACHE_TTL        Cache TTL in seconds (default: 3600)")
	fmt.Println("  TEST_CACHE_DIR        Cache directory (default: .test-cache)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Run all tests")
	fmt.Println("  go run ./api-toolkit/cmd/tester/main.go")
	fmt.Println()
	fmt.Println("  # Run specific test pattern")
	fmt.Println("  TEST_PATTERN=TestFoo go run ./api-toolkit/cmd/tester/main.go")
	fmt.Println()
	fmt.Println("  # Run with race detection")
	fmt.Println("  FLAGS='-race -count=1' go run ./api-toolkit/cmd/tester/main.go")
	fmt.Println()
	fmt.Println("  # Skip API wait")
	fmt.Println("  SKIP_API_WAIT=true go run ./api-toolkit/cmd/tester/main.go")
	fmt.Println()
	fmt.Println("  # Fast mode (no coverage, no caching)")
	fmt.Println("  FAST=true go run ./api-toolkit/cmd/tester/main.go")
	fmt.Println()
}

func computeCacheKey(pkg, pattern, flags string) string {
	sum := sha256.Sum256([]byte(pkg + "|" + pattern + "|" + flags))
	return hex.EncodeToString(sum[:])
}

func isCacheValid(cacheRoot *os.Root, cacheName string, ttlSeconds int) bool {
	if cacheRoot == nil {
		return false
	}
	st, err := cacheRoot.Stat(cacheName)
	if err != nil {
		return false
	}
	age := time.Since(st.ModTime())
	return age <= time.Duration(ttlSeconds)*time.Second
}

func sanitizeForFilename(s string) string {
	r := strings.NewReplacer("/", "--", ".", "--")
	return r.Replace(s)
}

func runGoCmdStreaming(ctx context.Context, args ...string) (int, error) {
	cmd := exec.CommandContext(normalizeContext(ctx), "go")
	cmd.Args = append(cmd.Args, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	var exitCode int
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			exitCode = 1
		}
		return exitCode, err
	}
	return 0, nil
}

func runGoList(ctx context.Context, pkgsPattern string) ([]string, error) {
	cmd := exec.CommandContext(normalizeContext(ctx), "go", "list")
	cmd.Args = append(cmd.Args, pkgsPattern)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list failed: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var pkgs []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			pkgs = append(pkgs, l)
		}
	}
	return pkgs, nil
}

func splitFlags(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	// Simple split; avoid bringing an extra dependency for shlex.
	return strings.Fields(s)
}

func waitForAPI(ctx context.Context, baseURL string, maxTotalWait time.Duration) error {
	if baseURL == "" {
		return fmt.Errorf("API_HOST not set")
	}
	ctx = normalizeContext(ctx)
	healthURL := strings.TrimRight(baseURL, "/") + "/health"
	client := &http.Client{Timeout: 2 * time.Second}
	fmt.Printf("Waiting for API server to be ready at %s\n", healthURL)

	initialBackoff := 1 * time.Second
	maxBackoff := 30 * time.Second
	var backoff time.Duration
	enforceMaxWait := maxTotalWait > 0

	// Exponential decay backoff: backoff = maxBackoff - (maxBackoff-initialBackoff)*decay^attempt
	decay := 0.95 // can tune for aggressiveness, 0 < decay < 1

	attempt := 0
	startTime := time.Now()

	for {
		// Check if total elapsed time exceeds maxTotalWait
		elapsed := time.Since(startTime)
		if enforceMaxWait && elapsed > maxTotalWait {
			return fmt.Errorf("timed out waiting for API after %s", maxTotalWait)
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			}
			fmt.Printf("API server not ready: %s\n", resp.Status)
		} else {
			fmt.Printf("Error requesting API health: %v\n", err)
		}

		// Calculate next backoff based on exponential decay
		d := float64(maxBackoff-initialBackoff) * pow(decay, float64(attempt))
		backoff = maxBackoff - time.Duration(d)
		if backoff < initialBackoff {
			backoff = initialBackoff
		}
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		// Don't sleep past maxTotalWait
		timeToNext := backoff
		if enforceMaxWait && elapsed+timeToNext > maxTotalWait {
			timeToNext = maxTotalWait - elapsed
		}
		fmt.Printf("Retrying in around %ds...\n", int64((timeToNext+time.Second/2)/time.Second))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(timeToNext):
			attempt++
		}
	}
}

// pow is a helper for integer durations and float exponents
func pow(a, b float64) float64 {
	return math.Pow(a, b)
}

func runTestsWithCache(ctx context.Context, pkg string, runPattern string, flags string, cfg *Config) error {
	cacheKey := computeCacheKey(pkg, runPattern, flags)
	var cacheRoot *os.Root
	if cfg.CacheEnabled {
		if err := os.MkdirAll(cfg.CacheDir, 0o750); err == nil {
			cacheRoot, _ = os.OpenRoot(cfg.CacheDir)
		}
	}
	if cacheRoot != nil {
		defer func() {
			_ = cacheRoot.Close()
		}()
	}

	if cfg.CacheEnabled && cacheRoot != nil && isCacheValid(cacheRoot, cacheKey, cfg.CacheTTL) {
		fmt.Printf("📦 Using cached test results: %s\n", pkg)
		data, err := readRootFile(cacheRoot, cacheKey)
		if err == nil {
			if _, err := os.Stdout.Write(data); err != nil {
				return err
			}
			return nil
		}
	}

	fmt.Printf("🧪 Running tests: %s\n", pkg)

	covDir := ".coverage"
	_ = os.MkdirAll(covDir, 0o750)
	covFile := filepath.Join(covDir, fmt.Sprintf("coverage.%s.out", sanitizeForFilename(pkg)))

	args := []string{"test", "-v", "-failfast", "-covermode=atomic", "-coverprofile=" + covFile}
	args = append(args, splitFlags(flags)...)
	if !cfg.CacheEnabled {
		args = append(args, "-count=1")
	}
	if strings.TrimSpace(runPattern) != "" {
		args = append(args, "-run", runPattern)
	}
	args = append(args, pkg)

	cmd := exec.CommandContext(normalizeContext(ctx), "go")
	cmd.Args = append(cmd.Args, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()

	// Always print collected output
	if _, err := os.Stdout.Write(buf.Bytes()); err != nil {
		return err
	}

	if err != nil {
		return err
	}

	if cfg.CacheEnabled && cacheRoot != nil {
		_ = writeRootFile(cacheRoot, cacheKey, buf.Bytes(), 0o600)
	}
	return nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func readRootFile(root *os.Root, name string) ([]byte, error) {
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	return io.ReadAll(f)
}

func writeRootFile(root *os.Root, name string, data []byte, perm os.FileMode) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	_, err = f.Write(data)
	return err
}

func main() {
	// Check for help flags
	if len(os.Args) > 1 {
		for _, arg := range os.Args[1:] {
			if arg == "-h" || arg == "--help" || arg == "help" {
				showHelp()
				return
			}
		}
	}

	// Load configuration
	cfg := LoadConfig()
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// API wait
	if !cfg.SkipAPIWait {
		if cfg.APIHost == "" {
			fatalf("❌ API_HOST environment variable is not set. Please define it in your env var file.")
		}
		if err := waitForAPI(rootCtx, cfg.APIHost, cfg.APIWaitTimeout); err != nil {
			fatalf("failed waiting for API: %v", err)
		}
	} else {
		fmt.Println("SKIP_API_WAIT set; not waiting for API")
	}

	fmt.Printf("Running tests with caching enabled: %v\n", cfg.CacheEnabled)

	// Expand packages
	pkgs, err := runGoList(rootCtx, cfg.PackagePattern)
	if err != nil {
		fatalf("%v", err)
	}

	// FAST mode: single go test invocation
	if cfg.FastMode {
		fmt.Println("FAST mode: single go test invocation, no race/coverage, no caching")
		args := []string{"test", "-v", "-failfast"}
		args = append(args, splitFlags(cfg.Flags)...)
		if !cfg.CacheEnabled {
			args = append(args, "-count=1")
		}
		if strings.TrimSpace(cfg.TestPattern) != "" {
			args = append(args, "-run", cfg.TestPattern)
		}
		args = append(args, pkgs...)
		code, err := runGoCmdStreaming(rootCtx, args...)
		if err != nil {
			os.Exit(code)
		}
		return
	}

	// Non-FAST: per-package with coverage and caching
	// Clean old per-package coverage files directory exists already
	_ = os.MkdirAll(".coverage", 0o750)

	var exitCode int
	for _, p := range pkgs {
		fmt.Printf(">> Testing %s\n", p)
		if err := runTestsWithCache(rootCtx, p, cfg.TestPattern, cfg.Flags, cfg); err != nil {
			fmt.Printf("❌ Tests failed in %s\n", p)
			exitCode = 1
			break
		}
	}

	fmt.Printf("Tests completed with exit code: %d\n", exitCode)
	os.Exit(exitCode)
}
