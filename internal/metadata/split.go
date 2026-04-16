// Package metadata — GenerateSplitMetadata creates Metadata for lib/bin/dev splits.
// NurOS 2026 - GPL 3.0
package metadata

import (
	"fmt"

	"github.com/NurOS-Linux/apgbuild/internal/elfanalyzer"
)

// SplitKind identifies which sub-package to generate metadata for.
type SplitKind int

const (
	SplitLibs SplitKind = iota // lib<name>
	SplitBins                  // <name>
	SplitDev                   // <name>-dev
)

// GenerateSplitMetadata creates a Metadata for a split sub-package.
//
//   - base:     metadata of the parent package (version, maintainer, license, etc.)
//   - splitDir: path to the split DESTDIR (used for ELF dep detection)
//   - kind:     SplitLibs | SplitBins | SplitDev
//   - baseName: base package name, e.g. "curl"
//
// Naming convention (Fedora-style):
//
//	SplitLibs → "lib<baseName>"
//	SplitBins → "<baseName>"
//	SplitDev  → "<baseName>-dev"
func GenerateSplitMetadata(base *Metadata, splitDir string, kind SplitKind, baseName string) (*Metadata, error) {
	m := &Metadata{
		Version:      base.Version,
		Architecture: base.Architecture,
		Maintainer:   base.Maintainer,
		License:      base.License,
		Homepage:     base.Homepage,
		Tags:         base.Tags,
		Conflicts:    []string{},
		Provides:     []string{},
		Replaces:     []string{},
		Conf:         []string{},
	}

	libName := "lib" + baseName

	switch kind {
	case SplitLibs:
		m.Name = libName
		m.Type = "library"
		m.Description = fmt.Sprintf("Shared libraries for %s", baseName)
		m.Dependencies = []string{}
		// Auto-detect runtime deps from ELF .so files
		if err := m.DetectDependenciesFromDir(splitDir); err != nil {
			return nil, fmt.Errorf("detect deps for %s: %w", m.Name, err)
		}
		m.Provides = []string{libName}

	case SplitBins:
		m.Name = baseName
		m.Type = "binary"
		m.Description = base.Description
		// Bins depend on the libs split + any additional ELF deps
		m.Dependencies = []string{libName}
		if err := addELFDeps(m, splitDir, []string{libName}); err != nil {
			return nil, fmt.Errorf("detect deps for %s: %w", m.Name, err)
		}

	case SplitDev:
		m.Name = baseName + "-dev"
		m.Type = "misc"
		m.Description = fmt.Sprintf("Development files for %s", baseName)
		m.Dependencies = []string{libName}
		m.Provides = []string{baseName + "-devel"}

	default:
		return nil, fmt.Errorf("unknown SplitKind: %d", kind)
	}

	return m, nil
}

// addELFDeps detects ELF dependencies from splitDir and merges them into m.Dependencies,
// skipping any already listed in skip.
func addELFDeps(m *Metadata, splitDir string, skip []string) error {
	libs, err := elfanalyzer.ExtractFromDir(splitDir)
	if err != nil {
		return err
	}
	pkgs := elfanalyzer.MapLibsToPackages(libs)

	skipSet := make(map[string]bool, len(skip))
	for _, s := range skip {
		skipSet[s] = true
	}
	existing := make(map[string]bool, len(m.Dependencies))
	for _, d := range m.Dependencies {
		existing[d] = true
	}

	for _, pkg := range pkgs {
		if !skipSet[pkg] && !existing[pkg] {
			m.Dependencies = append(m.Dependencies, pkg)
			existing[pkg] = true
		}
	}
	return nil
}
