// Package checksum provides CRC32 checksum functionality for APG packages.
// NurOS 2026 - GPL 3.0
package checksum

import (
	"bufio"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CRC32Table is the IEEE polynomial table used for CRC32 calculations.
var CRC32Table = crc32.MakeTable(crc32.IEEE)

// CalculateCRC32 computes the CRC32 checksum of a file.
func CalculateCRC32(filePath string) (uint32, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := crc32.New(CRC32Table)
	buf := make([]byte, 64*1024)

	for {
		n, err := file.Read(buf)
		if n > 0 {
			hash.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("failed to read file: %w", err)
		}
	}

	return hash.Sum32(), nil
}

// CalculateCRC32Bytes computes CRC32 checksum of a byte slice.
func CalculateCRC32Bytes(data []byte) uint32 {
	return crc32.Checksum(data, CRC32Table)
}

// Entry represents a single checksum entry.
type Entry struct {
	Checksum uint32
	Path     string
}

// CreateCRC32Sums generates crc32sums file for all files in directory.
func CreateCRC32Sums(directory, outputPath string) ([]Entry, error) {
	var entries []Entry

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(directory, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		checksum, err := CalculateCRC32(path)
		if err != nil {
			return fmt.Errorf("failed to calculate CRC32 for %s: %w", relPath, err)
		}

		entries = append(entries, Entry{
			Checksum: checksum,
			Path:     relPath,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Write to file
	file, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create crc32sums file: %w", err)
	}
	defer file.Close()

	for _, entry := range entries {
		_, err := fmt.Fprintf(file, "%08x  %s\n", entry.Checksum, entry.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to write checksum: %w", err)
		}
	}

	return entries, nil
}

// VerifyCRC32Sums verifies files against crc32sums file.
func VerifyCRC32Sums(sumsFile, baseDir string) ([]string, []string, error) {
	file, err := os.Open(sumsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open crc32sums: %w", err)
	}
	defer file.Close()

	var passed, failed []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			continue
		}

		var expectedSum uint32
		_, err := fmt.Sscanf(parts[0], "%x", &expectedSum)
		if err != nil {
			failed = append(failed, parts[1])
			continue
		}

		filePath := filepath.Join(baseDir, parts[1])
		actualSum, err := CalculateCRC32(filePath)
		if err != nil {
			failed = append(failed, parts[1])
			continue
		}

		if actualSum == expectedSum {
			passed = append(passed, parts[1])
		} else {
			failed = append(failed, parts[1])
		}
	}

	return passed, failed, scanner.Err()
}
