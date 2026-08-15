package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDocumentMetadataRejectsMalformedAndDuplicateRows(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("create docs directory: %v", err)
	}
	path := filepath.Join(docsDir, "document-owners.tsv")
	content := strings.Join([]string{
		"path\taudience\tcanonical_owner\tlifecycle",
		"docs/example.md\treaders\tdocs\tcurrent",
		"docs/example.md\treaders\tdocs\thistorical",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	_, err := loadDocumentMetadata(root)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("loadDocumentMetadata() error = %v, want duplicate row rejection", err)
	}
}

func TestDocumentCategoryMarksHistoricalRecords(t *testing.T) {
	if got := documentCategory("migration", "historical"); got != "historical" {
		t.Fatalf("historical category = %q, want historical", got)
	}
	if got := documentCategory("migration", "current"); got != "migration" {
		t.Fatalf("current category = %q, want migration", got)
	}
}
