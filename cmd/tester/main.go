package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/envvar"
)

var env = envvar.New()

// Config holds all configuration for the test runner
type Config struct {
	// API configuration
	APIHost     string
	SkipAPIWait bool

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
		SkipAPIWait:    env.GetOr("SKIP_API_WAIT", "") != "",
		PackagePattern: env.GetOr("PKG", "./..."),
		TestPattern:    env.GetOr("TEST_PATTERN", ""),
		Flags:          env.GetOr("FLAGS", ""),
		FastMode:       env.GetOr("FAST", "") != "",
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
	h := md5.Sum([]byte(pkg + "|" + pattern + "|" + flags))
	return hex.EncodeToString(h[:])
}

func isCacheValid(cacheFile string, ttlSeconds int) bool {
	st, err := os.Stat(cacheFile)
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

func runCmdStreaming(name string, args ...string) (int, error) {
	cmd := exec.Command(name, args...)
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

func runGoList(pkgsPattern string) ([]string, error) {
	cmd := exec.Command("go", "list", pkgsPattern)
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

func waitForAPI(ctx context.Context, baseURL string) error {
	if baseURL == "" {
		return fmt.Errorf("API_HOST not set")
	}
	healthURL := strings.TrimRight(baseURL, "/") + "/health"
	client := &http.Client{Timeout: 2 * time.Second}
	fmt.Printf("Waiting for API server to be ready at %s\n", healthURL)

	initialBackoff := 1 * time.Second
	maxBackoff := 30 * time.Second
	maxTotalWait := 20 * time.Second
	backoff := time.Duration(0)

	// Exponential decay backoff: backoff = maxBackoff - (maxBackoff-initialBackoff)*decay^attempt
	decay := 0.95 // can tune for aggressiveness, 0 < decay < 1

	attempt := 0
	startTime := time.Now()

	for {
		// Check if total elapsed time exceeds maxTotalWait
		elapsed := time.Since(startTime)
		if elapsed > maxTotalWait {
			return fmt.Errorf("timed out waiting for API after %s", maxTotalWait)
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			} else {
				fmt.Printf("API server not ready: %s\n", resp.Status)
			}
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
		if elapsed+timeToNext > maxTotalWait {
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

func runTestsWithCache(pkg string, runPattern string, flags string, cfg *Config) error {
	cacheKey := computeCacheKey(pkg, runPattern, flags)
	cacheFile := filepath.Join(cfg.CacheDir, cacheKey)

	if cfg.CacheEnabled && isCacheValid(cacheFile, cfg.CacheTTL) {
		fmt.Printf("📦 Using cached test results for %s\n", pkg)
		data, _ := os.ReadFile(cacheFile)
		os.Stdout.Write(data)
		return nil
	}

	fmt.Printf("🧪 Running tests for %s (not cached or cache expired)\n", pkg)

	covDir := ".coverage"
	_ = os.MkdirAll(covDir, 0o755)
	covFile := filepath.Join(covDir, fmt.Sprintf("coverage.%s.out", sanitizeForFilename(pkg)))

	args := []string{"test", "-v", "-failfast", "-covermode=atomic", "-coverprofile=" + covFile}
	args = append(args, splitFlags(flags)...)
	if strings.TrimSpace(runPattern) != "" {
		args = append(args, "-run", runPattern)
	}
	args = append(args, pkg)

	cmd := exec.Command("go", args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()

	// Always print collected output
	os.Stdout.Write(buf.Bytes())

	if err != nil {
		return err
	}

	if cfg.CacheEnabled {
		_ = os.MkdirAll(cfg.CacheDir, 0o755)
		_ = os.WriteFile(cacheFile, buf.Bytes(), 0o644)
	}
	return nil
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

	// API wait
	if !cfg.SkipAPIWait {
		if cfg.APIHost == "" {
			fatalf("❌ API_HOST environment variable is not set. Please define it in your .env or .env.test file.")
		}
		// No timeout to match original script behavior (wait indefinitely)
		if err := waitForAPI(context.Background(), cfg.APIHost); err != nil {
			fatalf("failed waiting for API: %v", err)
		}
	} else {
		fmt.Println("SKIP_API_WAIT set; not waiting for API")
	}

	fmt.Printf("Running tests with caching enabled: %v\n", cfg.CacheEnabled)

	// Expand packages
	pkgs, err := runGoList(cfg.PackagePattern)
	if err != nil {
		fatalf("%v", err)
	}

	// FAST mode: single go test invocation
	if cfg.FastMode {
		fmt.Println("FAST mode: single go test invocation, no race/coverage, no caching")
		args := []string{"test", "-v", "-failfast"}
		args = append(args, splitFlags(cfg.Flags)...)
		if strings.TrimSpace(cfg.TestPattern) != "" {
			args = append(args, "-run", cfg.TestPattern)
		}
		args = append(args, pkgs...)
		code, err := runCmdStreaming("go", args...)
		if err != nil {
			os.Exit(code)
		}
		return
	}

	// Non-FAST: per-package with coverage and caching
	// Clean old per-package coverage files directory exists already
	_ = os.MkdirAll(".coverage", 0o755)

	var exitCode int
	for _, p := range pkgs {
		fmt.Printf(">> Testing %s\n", p)
		if err := runTestsWithCache(p, cfg.TestPattern, cfg.Flags, cfg); err != nil {
			fmt.Printf("❌ Tests failed in %s\n", p)
			exitCode = 1
			break
		}
	}

	fmt.Printf("Tests completed with exit code: %d\n", exitCode)
	os.Exit(exitCode)
}
