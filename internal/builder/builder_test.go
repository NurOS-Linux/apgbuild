package builder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	b := New()
	if b == nil {
		t.Fatal("New() returned nil")
	}
}

func TestCreatePackage(t *testing.T) {
	b := New()

	// Create temp source directory
	sourceDir := t.TempDir()
	dataDir := filepath.Join(sourceDir, "data", "usr", "bin")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}

	// Create test file
	testFile := filepath.Join(dataDir, "hello")
	if err := os.WriteFile(testFile, []byte("#!/bin/bash\necho hello"), 0755); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create metadata.json
	metaPath := filepath.Join(sourceDir, "metadata.json")
	metaContent := `{
		"name": "test",
		"version": "1.0.0",
		"type": "misc",
		"architecture": null,
		"description": "Test package",
		"maintainer": "Test",
		"license": null,
		"tags": [],
		"homepage": "",
		"dependencies": [],
		"conflicts": [],
		"provides": [],
		"replaces": [],
		"conf": []
	}`
	if err := os.WriteFile(metaPath, []byte(metaContent), 0644); err != nil {
		t.Fatalf("Failed to create metadata: %v", err)
	}

	// Create package
	outputPath := filepath.Join(t.TempDir(), "test.apg")
	if err := b.CreatePackage(sourceDir, outputPath); err != nil {
		t.Fatalf("CreatePackage failed: %v", err)
	}

	// Verify package exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Package was not created")
	}

	// Verify crc32sums was created
	sumsPath := filepath.Join(sourceDir, "crc32sums")
	if _, err := os.Stat(sumsPath); os.IsNotExist(err) {
		t.Error("crc32sums was not created")
	}
}

func TestCreatePackage_NonExistent(t *testing.T) {
	b := New()
	err := b.CreatePackage("/nonexistent/dir", "/tmp/test.apg")
	if err == nil {
		t.Error("CreatePackage should fail for non-existent directory")
	}
}

func TestExtractPackage(t *testing.T) {
	b := New()

	// First create a package
	sourceDir := t.TempDir()
	dataDir := filepath.Join(sourceDir, "data")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}

	testFile := filepath.Join(dataDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	packagePath := filepath.Join(t.TempDir(), "test.apg")
	if err := b.CreatePackage(sourceDir, packagePath); err != nil {
		t.Fatalf("CreatePackage failed: %v", err)
	}

	// Extract package
	destDir := t.TempDir()
	if err := b.ExtractPackageTo(packagePath, destDir); err != nil {
		t.Fatalf("ExtractPackageTo failed: %v", err)
	}

	// Verify extraction
	extractedFile := filepath.Join(destDir, "data", "test.txt")
	if _, err := os.Stat(extractedFile); os.IsNotExist(err) {
		t.Error("File was not extracted")
	}
}

func TestExtractPackage_NonExistent(t *testing.T) {
	b := New()
	err := b.ExtractPackage("/nonexistent/package.apg")
	if err == nil {
		t.Error("ExtractPackage should fail for non-existent package")
	}
}

func TestGenerateChecksums(t *testing.T) {
	b := New()

	// Create temp directory with files
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Generate checksums
	outputPath := filepath.Join(t.TempDir(), "crc32sums")
	if err := b.GenerateChecksums(tmpDir, outputPath); err != nil {
		t.Fatalf("GenerateChecksums failed: %v", err)
	}

	// Verify output exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("crc32sums was not created")
	}
}

func TestGenerateChecksums_NonExistent(t *testing.T) {
	b := New()
	err := b.GenerateChecksums("/nonexistent/dir", "/tmp/sums")
	if err == nil {
		t.Error("GenerateChecksums should fail for non-existent directory")
	}
}

func TestVerifyChecksums(t *testing.T) {
	b := New()

	// Create temp directory with files
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Generate checksums
	sumsPath := filepath.Join(tmpDir, "crc32sums")
	if err := b.GenerateChecksums(tmpDir, sumsPath); err != nil {
		t.Fatalf("GenerateChecksums failed: %v", err)
	}

	// Verify checksums (should pass)
	if err := b.VerifyChecksums(sumsPath, tmpDir); err != nil {
		t.Fatalf("VerifyChecksums failed: %v", err)
	}

	// Modify file
	if err := os.WriteFile(testFile, []byte("modified content"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Verify checksums (should fail)
	if err := b.VerifyChecksums(sumsPath, tmpDir); err == nil {
		t.Error("VerifyChecksums should fail after file modification")
	}
}

func TestListPackage(t *testing.T) {
	b := New()

	// Create a package
	sourceDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(sourceDir, "file1.txt"), []byte("1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "file2.txt"), []byte("2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	packagePath := filepath.Join(t.TempDir(), "test.apg")
	if err := b.CreatePackage(sourceDir, packagePath); err != nil {
		t.Fatalf("CreatePackage failed: %v", err)
	}

	// List package
	if err := b.ListPackage(packagePath); err != nil {
		t.Fatalf("ListPackage failed: %v", err)
	}
}
