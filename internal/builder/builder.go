// Package builder provides the main APG package building functionality.
// NurOS 2026 - GPL 3.0
package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/NurOS-Linux/apgbuild/internal/archive"
	"github.com/NurOS-Linux/apgbuild/internal/checksum"
	"github.com/NurOS-Linux/apgbuild/internal/metadata"
)

// Color codes for terminal output.
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
	ColorBold   = "\033[1m"
)

// Builder handles APG package building operations.
type Builder struct{}

// New creates a new Builder instance.
func New() *Builder {
	return &Builder{}
}

// CreatePackage creates an APG package from a directory.
func (b *Builder) CreatePackage(sourceDir, outputPath string) error {
	fmt.Printf("%sCreating package from directory: %s%s\n", ColorCyan, sourceDir, ColorReset)

	// Validate source directory
	info, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("source directory does not exist: %s", sourceDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", sourceDir)
	}

	// Check for required metadata.json
	metadataPath := filepath.Join(sourceDir, "metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		fmt.Printf("%sWarning: metadata.json not found in package%s\n", ColorYellow, ColorReset)
	} else {
		// Validate metadata
		meta, err := metadata.Load(metadataPath)
		if err != nil {
			fmt.Printf("%sWarning: failed to parse metadata.json: %v%s\n", ColorYellow, err, ColorReset)
		} else if err := meta.Validate(); err != nil {
			fmt.Printf("%sWarning: metadata validation failed: %v%s\n", ColorYellow, err, ColorReset)
		}
	}

	// Generate CRC32 checksums for data directory
	dataDir := filepath.Join(sourceDir, "data")
	if info, err := os.Stat(dataDir); err == nil && info.IsDir() {
		sumsPath := filepath.Join(sourceDir, "crc32sums")
		fmt.Printf("%sGenerating CRC32 checksums for data directory...%s\n", ColorCyan, ColorReset)

		entries, err := checksum.CreateCRC32Sums(dataDir, sumsPath)
		if err != nil {
			return fmt.Errorf("failed to create checksums: %w", err)
		}

		for _, entry := range entries {
			fmt.Printf("%s  %s%s\n", ColorGreen, entry.Path, ColorReset)
		}
		fmt.Printf("%sGenerated %d checksums%s\n", ColorGreen, len(entries), ColorReset)
	}

	// Generate checksums for home directory if exists
	homeDir := filepath.Join(sourceDir, "home")
	if info, err := os.Stat(homeDir); err == nil && info.IsDir() {
		sumsPath := filepath.Join(sourceDir, "crc32sums.home")
		fmt.Printf("%sGenerating CRC32 checksums for home directory...%s\n", ColorCyan, ColorReset)

		entries, err := checksum.CreateCRC32Sums(homeDir, sumsPath)
		if err != nil {
			fmt.Printf("%sWarning: failed to create home checksums: %v%s\n", ColorYellow, err, ColorReset)
		} else {
			fmt.Printf("%sGenerated %d home checksums%s\n", ColorGreen, len(entries), ColorReset)
		}
	}

	// Create archive
	fmt.Printf("%sCreating archive...%s\n", ColorCyan, ColorReset)
	result, err := archive.Create(outputPath, sourceDir)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	fmt.Printf("%s Package created successfully: %s%s\n", ColorGreen, outputPath, ColorReset)
	fmt.Printf("%s  Files: %d, Size: %d bytes%s\n", ColorGreen, result.FilesAdded, result.TotalSize, ColorReset)

	return nil
}

// ExtractPackage extracts an APG package to the current directory.
func (b *Builder) ExtractPackage(packagePath string) error {
	fmt.Printf("%sExtracting package: %s%s\n", ColorCyan, packagePath, ColorReset)

	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return fmt.Errorf("package not found: %s", packagePath)
	}

	destDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if err := archive.Extract(packagePath, destDir); err != nil {
		return fmt.Errorf("failed to extract package: %w", err)
	}

	fmt.Printf("%s Package extracted successfully%s\n", ColorGreen, ColorReset)
	return nil
}

// ExtractPackageTo extracts an APG package to a specified directory.
func (b *Builder) ExtractPackageTo(packagePath, destDir string) error {
	fmt.Printf("%sExtracting package: %s to %s%s\n", ColorCyan, packagePath, destDir, ColorReset)

	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return fmt.Errorf("package not found: %s", packagePath)
	}

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	if err := archive.Extract(packagePath, destDir); err != nil {
		return fmt.Errorf("failed to extract package: %w", err)
	}

	fmt.Printf("%s Package extracted successfully%s\n", ColorGreen, ColorReset)
	return nil
}

// CreateMetadata runs the interactive metadata creation wizard.
func (b *Builder) CreateMetadata(outputPath string) error {
	wizard := metadata.NewWizard()

	meta, err := wizard.Run()
	if err != nil {
		return fmt.Errorf("metadata creation failed: %w", err)
	}

	if err := meta.Save(outputPath); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	fmt.Printf("%s metadata.json created successfully!%s\n", ColorGreen, ColorReset)
	return nil
}

// GenerateChecksums generates CRC32 checksums for a directory.
func (b *Builder) GenerateChecksums(directory, outputPath string) error {
	fmt.Printf("%sGenerating CRC32 checksums for: %s%s\n", ColorCyan, directory, ColorReset)

	info, err := os.Stat(directory)
	if err != nil {
		return fmt.Errorf("directory does not exist: %s", directory)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", directory)
	}

	entries, err := checksum.CreateCRC32Sums(directory, outputPath)
	if err != nil {
		return fmt.Errorf("failed to generate checksums: %w", err)
	}

	for _, entry := range entries {
		fmt.Printf("%s  %08x  %s%s\n", ColorGreen, entry.Checksum, entry.Path, ColorReset)
	}

	fmt.Printf("%s Generated %d checksums to %s%s\n", ColorGreen, len(entries), outputPath, ColorReset)
	return nil
}

// VerifyChecksums verifies files against a crc32sums file.
func (b *Builder) VerifyChecksums(sumsFile, baseDir string) error {
	fmt.Printf("%sVerifying checksums from: %s%s\n", ColorCyan, sumsFile, ColorReset)

	passed, failed, err := checksum.VerifyCRC32Sums(sumsFile, baseDir)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	for _, f := range passed {
		fmt.Printf("%s  %s%s\n", ColorGreen, f, ColorReset)
	}

	for _, f := range failed {
		fmt.Printf("%s  %s%s\n", ColorRed, f, ColorReset)
	}

	fmt.Printf("\n%sPassed: %d, Failed: %d%s\n", ColorCyan, len(passed), len(failed), ColorReset)

	if len(failed) > 0 {
		return fmt.Errorf("%d files failed verification", len(failed))
	}

	return nil
}

// ListPackage lists the contents of an APG package.
func (b *Builder) ListPackage(packagePath string) error {
	fmt.Printf("%sListing contents of: %s%s\n", ColorCyan, packagePath, ColorReset)

	contents, err := archive.ListContents(packagePath)
	if err != nil {
		return fmt.Errorf("failed to list package: %w", err)
	}

	for _, entry := range contents {
		fmt.Printf("  %s\n", entry)
	}

	fmt.Printf("\n%sTotal: %d entries%s\n", ColorCyan, len(contents), ColorReset)
	return nil
}
