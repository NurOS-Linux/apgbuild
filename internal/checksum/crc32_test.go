package checksum

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateCRC32Bytes(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected uint32
	}{
		{
			name:     "empty",
			data:     []byte{},
			expected: 0,
		},
		{
			name:     "hello",
			data:     []byte("hello"),
			expected: 0x3610a686,
		},
		{
			name:     "hello world",
			data:     []byte("hello world"),
			expected: 0x0d4a1185,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateCRC32Bytes(tt.data)
			if result != tt.expected {
				t.Errorf("CalculateCRC32Bytes(%q) = %08x, want %08x", tt.data, result, tt.expected)
			}
		})
	}
}

func TestCalculateCRC32(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("test content for crc32")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Calculate expected CRC32
	expected := CalculateCRC32Bytes(content)

	// Test file CRC32
	result, err := CalculateCRC32(testFile)
	if err != nil {
		t.Fatalf("CalculateCRC32 failed: %v", err)
	}

	if result != expected {
		t.Errorf("CalculateCRC32(%q) = %08x, want %08x", testFile, result, expected)
	}
}

func TestCalculateCRC32_NonExistent(t *testing.T) {
	_, err := CalculateCRC32("/nonexistent/file.txt")
	if err == nil {
		t.Error("CalculateCRC32 should fail for non-existent file")
	}
}

func TestCreateCRC32Sums(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "subdir", "file2.txt")

	if err := os.MkdirAll(filepath.Dir(file2), 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Create checksums
	outputPath := filepath.Join(tmpDir, "crc32sums")
	entries, err := CreateCRC32Sums(tmpDir, outputPath)
	if err != nil {
		t.Fatalf("CreateCRC32Sums failed: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}

	// Verify output file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("crc32sums file was not created")
	}
}

func TestVerifyCRC32Sums(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create checksums
	sumsFile := filepath.Join(tmpDir, "crc32sums")
	if _, err := CreateCRC32Sums(tmpDir, sumsFile); err != nil {
		t.Fatalf("CreateCRC32Sums failed: %v", err)
	}

	// Verify checksums
	passed, failed, err := VerifyCRC32Sums(sumsFile, tmpDir)
	if err != nil {
		t.Fatalf("VerifyCRC32Sums failed: %v", err)
	}

	if len(passed) != 1 {
		t.Errorf("Expected 1 passed, got %d", len(passed))
	}
	if len(failed) != 0 {
		t.Errorf("Expected 0 failed, got %d", len(failed))
	}

	// Modify file and verify again
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	passed, failed, err = VerifyCRC32Sums(sumsFile, tmpDir)
	if err != nil {
		t.Fatalf("VerifyCRC32Sums failed: %v", err)
	}

	if len(passed) != 0 {
		t.Errorf("Expected 0 passed after modification, got %d", len(passed))
	}
	if len(failed) != 1 {
		t.Errorf("Expected 1 failed after modification, got %d", len(failed))
	}
}
