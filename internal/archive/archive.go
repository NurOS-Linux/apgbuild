// Package archive provides tar.xz archive creation and extraction for APG packages.
// NurOS 2026 - GPL 3.0
package archive

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

const (
	// MaxFileSize is the maximum allowed file size (100 MB).
	MaxFileSize = 100 * 1024 * 1024
	// MaxArchiveSize is the maximum allowed total archive size (1 GB).
	MaxArchiveSize = 1024 * 1024 * 1024
	// MaxFiles is the maximum number of files in an archive.
	MaxFiles = 10000
)

// CreateResult contains information about created archive.
type CreateResult struct {
	FilesAdded int
	TotalSize  int64
}

// Create creates a tar.xz archive from a directory.
func Create(archivePath, sourceDir string) (*CreateResult, error) {
	// Create output file
	outFile, err := os.Create(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive file: %w", err)
	}
	defer outFile.Close()

	// Create XZ writer
	xzWriter, err := xz.NewWriter(outFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create xz writer: %w", err)
	}
	defer xzWriter.Close()

	// Create TAR writer
	tarWriter := tar.NewWriter(xzWriter)
	defer tarWriter.Close()

	result := &CreateResult{}

	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}

		header.Name = relPath

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink: %w", err)
			}
			header.Linkname = link
		}

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		// Write file content for regular files
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()

			written, err := io.Copy(tarWriter, file)
			if err != nil {
				return fmt.Errorf("failed to write file content: %w", err)
			}

			result.TotalSize += written
		}

		result.FilesAdded++
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// Extract extracts a tar.xz archive to a directory.
func Extract(archivePath, destDir string) error {
	// Open archive file
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	// Create XZ reader
	xzReader, err := xz.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create xz reader: %w", err)
	}

	// Create TAR reader
	tarReader := tar.NewReader(xzReader)

	var totalSize int64
	var fileCount int

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Security checks
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
			return fmt.Errorf("too many files in archive (>%d limit)", MaxFiles)
		}

		targetPath := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			outFile.Close()

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				// Ignore if symlink already exists
				if !os.IsExist(err) {
					fmt.Printf("\033[33mWarning: failed to create symlink %s: %v\033[0m\n", header.Name, err)
				}
			}

		case tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			linkPath := filepath.Join(destDir, header.Linkname)
			if err := os.Link(linkPath, targetPath); err != nil {
				fmt.Printf("\033[33mWarning: failed to create hard link %s: %v\033[0m\n", header.Name, err)
			}
		}
	}

	return nil
}

// isPathSafe checks if a path is safe to extract (no path traversal).
func isPathSafe(path, baseDir string) bool {
	if path == "" {
		return false
	}

	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return false
	}

	// Check for absolute paths
	if filepath.IsAbs(path) {
		return false
	}

	// Check for null bytes
	if strings.ContainsRune(path, 0) {
		return false
	}

	// Verify the resolved path is within baseDir
	fullPath := filepath.Join(baseDir, path)
	cleanPath := filepath.Clean(fullPath)

	return strings.HasPrefix(cleanPath, filepath.Clean(baseDir))
}

// ListContents lists the contents of a tar.xz archive.
func ListContents(archivePath string) ([]string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	xzReader, err := xz.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create xz reader: %w", err)
	}

	tarReader := tar.NewReader(xzReader)

	var contents []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}
		contents = append(contents, header.Name)
	}

	return contents, nil
}
