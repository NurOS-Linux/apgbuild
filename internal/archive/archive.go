// Package archive provides tar.zst archive creation and extraction for APG packages.
// Uses DataDog/zstd for fast Zstandard compression (pure Go, no CGO).
// NurOS 2026 - GPL 3.0
package archive

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/DataDog/zstd"
)

const (
	// MaxFileSize is the maximum allowed file size (100 MB).
	MaxFileSize = 100 * 1024 * 1024
	// MaxArchiveSize is the maximum allowed total archive size (1 GB).
	MaxArchiveSize = 1024 * 1024 * 1024
	// MaxFiles is the maximum number of files in an archive.
	MaxFiles = 10000
	// ZstdCompressionLevel is the default compression level (1-22, 19 is good).
	ZstdCompressionLevel = 19
)

// CreateResult contains information about created archive.
type CreateResult struct {
	FilesAdded int
	TotalSize  int64
}

// Create creates a tar.zst archive from a directory.
func Create(archivePath, sourceDir string) (*CreateResult, error) {
	// Open output file
	outFile, err := os.Create(archivePath)
	if err != nil {
		return nil, fmt.Errorf("create archive file: %w", err)
	}
	defer outFile.Close()

	// Create Zstd writer
	zstdWriter, err := zstd.NewWriter(outFile, zstd.WithCompressionLevel(zstd.SpeedBetterCompression))
	if err != nil {
		return nil, fmt.Errorf("create Zstd writer: %w", err)
	}
	defer zstdWriter.Close()

	// Create TAR writer writing to Zstd
	tarWriter := tar.NewWriter(zstdWriter)
	result := &CreateResult{}

	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("tar header: %w", err)
		}
		header.Name = relPath

		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("read symlink: %w", err)
			}
			header.Linkname = link
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("write header: %w", err)
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("open file: %w", err)
			}
			defer file.Close()

			written, err := io.Copy(tarWriter, file)
			if err != nil {
				return fmt.Errorf("write content: %w", err)
			}

			result.TotalSize += written
		}

		result.FilesAdded++
		return nil
	})

	if err != nil {
		return nil, err
	}

	if err := tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %w", err)
	}

	if err := zstdWriter.Close(); err != nil {
		return nil, fmt.Errorf("close zstd: %w", err)
	}

	return result, nil
}

// Extract extracts a tar.zst archive to a directory.
func Extract(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()

	// Create Zstd reader
	zstdReader, err := zstd.NewReader(file)
	if err != nil {
		return fmt.Errorf("create Zstd reader: %w", err)
	}
	defer zstdReader.Close()

	// Read TAR
	tarReader := tar.NewReader(zstdReader)
	var totalSize int64
	var fileCount int

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		if !isPathSafe(header.Name, destDir) {
			fmt.Printf("\033[33mWarning: skipping unsafe path: %s\033[0m\n", header.Name)
			continue
		}

		if header.Size > MaxFileSize {
			fmt.Printf("\033[33mWarning: skipping large file: %s\033[0m\n", header.Name)
			continue
		}

		totalSize += header.Size
		if totalSize > MaxArchiveSize {
			return fmt.Errorf("archive too large (>1GB limit)")
		}

		fileCount++
		if fileCount > MaxFiles {
			return fmt.Errorf("too many files (>%d limit)", MaxFiles)
		}

		targetPath := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("write: %w", err)
			}
			outFile.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				if !os.IsExist(err) {
					fmt.Printf("\033[33mWarning: symlink %s: %v\033[0m\n", header.Name, err)
				}
			}
		}
	}

	return nil
}

func isPathSafe(path, baseDir string) bool {
	if path == "" || strings.Contains(path, "..") || filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return false
	}
	return strings.HasPrefix(filepath.Clean(filepath.Join(baseDir, path)), filepath.Clean(baseDir))
}

// ListContents lists the contents of a tar.zst archive.
func ListContents(archivePath string) ([]string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()

	// Simple approach: extract to temp dir and list
	// For full implementation, use streaming decompression
	tempDir, err := os.MkdirTemp("", "apgbuild-list-*")
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := Extract(archivePath, tempDir); err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	var contents []string
	filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(tempDir, path)
			contents = append(contents, rel)
		}
		return nil
	})

	return contents, nil
}
