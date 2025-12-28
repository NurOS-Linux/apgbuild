package archive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndExtract(t *testing.T) {
	// Create temp directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(sourceDir, "test.txt")
	subDir := filepath.Join(sourceDir, "subdir")
	subFile := filepath.Join(subDir, "nested.txt")

	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := os.WriteFile(subFile, []byte("nested content"), 0644); err != nil {
		t.Fatalf("Failed to create nested file: %v", err)
	}

	// Create archive
	archivePath := filepath.Join(t.TempDir(), "test.tar.xz")
	result, err := Create(archivePath, sourceDir)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if result.FilesAdded < 2 {
		t.Errorf("Expected at least 2 files added, got %d", result.FilesAdded)
	}

	// Extract archive
	if err := Extract(archivePath, destDir); err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Verify extracted files
	extractedFile := filepath.Join(destDir, "test.txt")
	if _, err := os.Stat(extractedFile); os.IsNotExist(err) {
		t.Error("test.txt was not extracted")
	}

	extractedNested := filepath.Join(destDir, "subdir", "nested.txt")
	if _, err := os.Stat(extractedNested); os.IsNotExist(err) {
		t.Error("subdir/nested.txt was not extracted")
	}

	// Verify content
	content, err := os.ReadFile(extractedFile)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}

	if string(content) != "test content" {
		t.Errorf("Content mismatch: got %q, want %q", string(content), "test content")
	}
}

func TestListContents(t *testing.T) {
	sourceDir := t.TempDir()

	// Create test files
	if err := os.WriteFile(filepath.Join(sourceDir, "file1.txt"), []byte("1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "file2.txt"), []byte("2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Create archive
	archivePath := filepath.Join(t.TempDir(), "test.tar.xz")
	if _, err := Create(archivePath, sourceDir); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// List contents
	contents, err := ListContents(archivePath)
	if err != nil {
		t.Fatalf("ListContents failed: %v", err)
	}

	if len(contents) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(contents))
	}
}

func TestIsPathSafe(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		baseDir  string
		expected bool
	}{
		{
			name:     "safe relative path",
			path:     "file.txt",
			baseDir:  "/tmp/test",
			expected: true,
		},
		{
			name:     "safe nested path",
			path:     "dir/subdir/file.txt",
			baseDir:  "/tmp/test",
			expected: true,
		},
		{
			name:     "path traversal attempt",
			path:     "../etc/passwd",
			baseDir:  "/tmp/test",
			expected: false,
		},
		{
			name:     "absolute path",
			path:     "/etc/passwd",
			baseDir:  "/tmp/test",
			expected: false,
		},
		{
			name:     "empty path",
			path:     "",
			baseDir:  "/tmp/test",
			expected: false,
		},
		{
			name:     "hidden path traversal",
			path:     "foo/../../etc/passwd",
			baseDir:  "/tmp/test",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPathSafe(tt.path, tt.baseDir)
			if result != tt.expected {
				t.Errorf("isPathSafe(%q, %q) = %v, want %v", tt.path, tt.baseDir, result, tt.expected)
			}
		})
	}
}

func TestExtract_NonExistent(t *testing.T) {
	err := Extract("/nonexistent/archive.tar.xz", t.TempDir())
	if err == nil {
		t.Error("Extract should fail for non-existent archive")
	}
}

func TestCreate_NonExistentSource(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "test.tar.xz")
	_, err := Create(archivePath, "/nonexistent/directory")
	if err == nil {
		t.Error("Create should fail for non-existent source directory")
	}
}
