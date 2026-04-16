// Package checksum provides SHA-256 checksum functionality for APG packages.
// NurOS 2026 - GPL 3.0
package checksum

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Entry represents a single checksum entry.
type Entry struct {
	Checksum string // hex-encoded SHA-256
	Path     string
}

// Calculate computes the SHA-256 checksum of a file.
func Calculate(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CreateSums generates a sha256sums file for all files in directory.
func CreateSums(directory, outputPath string) ([]Entry, error) {
	var entries []Entry

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		sum, err := Calculate(path)
		if err != nil {
			return fmt.Errorf("sha256 %s: %w", rel, err)
		}
		entries = append(entries, Entry{Checksum: sum, Path: rel})
		return nil
	})
	if err != nil {
		return nil, err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("create sums file: %w", err)
	}
	defer f.Close()

	for _, e := range entries {
		fmt.Fprintf(f, "%s  %s\n", e.Checksum, e.Path)
	}
	return entries, nil
}

// VerifySums verifies files against a sha256sums file.
// Returns lists of passed and failed file paths.
func VerifySums(sumsFile, baseDir string) (passed, failed []string, err error) {
	f, err := os.Open(sumsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("open sums file: %w", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			continue
		}
		expected, relPath := parts[0], parts[1]
		actual, err := Calculate(filepath.Join(baseDir, relPath))
		if err != nil || actual != expected {
			failed = append(failed, relPath)
		} else {
			passed = append(passed, relPath)
		}
	}
	return passed, failed, sc.Err()
}

// ── Legacy aliases kept for any callers that used the old CRC32 names ─────────

// CreateCRC32Sums is a compatibility alias for CreateSums.
// Deprecated: use CreateSums — output is now SHA-256, not CRC32.
func CreateCRC32Sums(directory, outputPath string) ([]Entry, error) {
	return CreateSums(directory, outputPath)
}

// VerifyCRC32Sums is a compatibility alias for VerifySums.
// Deprecated: use VerifySums.
func VerifyCRC32Sums(sumsFile, baseDir string) ([]string, []string, error) {
	return VerifySums(sumsFile, baseDir)
}
