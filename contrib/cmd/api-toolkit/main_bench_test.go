package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkNewServiceSaaSAPIGeneration(b *testing.B) {
	repoRoot := benchmarkRepoRoot(b)
	tmp := b.TempDir()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		serviceDir := filepath.Join(tmp, fmt.Sprintf("service-%06d", i))
		var out strings.Builder
		code := run(context.Background(), []string{
			"new", "service",
			"--module", "example.com/my-api",
			"--profile", "saas-api",
			"--dir", serviceDir,
			"--core-replace", repoRoot,
			"--contrib-replace", filepath.Join(repoRoot, "contrib"),
		}, &out, &out)
		if code != 0 {
			b.Fatalf("new service failed: %s", out.String())
		}
		if _, err := os.Stat(filepath.Join(serviceDir, "go.mod")); err != nil {
			b.Fatalf("generated go.mod missing: %v", err)
		}
	}
}

func benchmarkRepoRoot(b *testing.B) string {
	b.Helper()
	wd, err := os.Getwd()
	if err != nil {
		b.Fatalf("getwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		b.Fatalf("repo root from %s: %v", wd, err)
	}
	return root
}
