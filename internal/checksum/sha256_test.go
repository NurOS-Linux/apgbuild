// Package checksum — SHA-256 tests.
// NurOS 2026 - GPL 3.0
package checksum

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculate(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	content := []byte("hello apger")
	os.WriteFile(path, content, 0644)

	sum, err := Calculate(path)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if len(sum) != 64 {
		t.Errorf("expected 64-char hex SHA-256, got %d chars: %s", len(sum), sum)
	}
}

func TestCreateAndVerifySums(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bbb"), 0644)

	sumsPath := filepath.Join(t.TempDir(), "sha256sums")
	entries, err := CreateSums(dir, sumsPath)
	if err != nil {
		t.Fatalf("CreateSums: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	passed, failed, err := VerifySums(sumsPath, dir)
	if err != nil {
		t.Fatalf("VerifySums: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected 0 failures, got %v", failed)
	}
	if len(passed) != 2 {
		t.Errorf("expected 2 passed, got %d", len(passed))
	}
}

func TestVerifySums_Tampered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	os.WriteFile(path, []byte("original"), 0644)

	sumsPath := filepath.Join(t.TempDir(), "sha256sums")
	CreateSums(dir, sumsPath)

	// Tamper with file
	os.WriteFile(path, []byte("tampered"), 0644)

	_, failed, _ := VerifySums(sumsPath, dir)
	if len(failed) != 1 {
		t.Errorf("expected 1 failure after tampering, got %d", len(failed))
	}
}
