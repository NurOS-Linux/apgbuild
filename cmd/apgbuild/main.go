// apgbuild — APG package builder CLI
// NurOS 2026 - GPL 3.0
//
// Commands:
//
//	build <dir> -o <out.apg>          — create APG package from directory
//	meta [-o metadata.json] [flags]   — generate or edit metadata.json
//	sums <dir> <output>               — generate CRC32 checksums
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/NurOS-Linux/apgbuild/internal/builder"
	"github.com/NurOS-Linux/apgbuild/internal/checksum"
	"github.com/NurOS-Linux/apgbuild/internal/metadata"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "build":
		err = cmdBuild(os.Args[2:])
	case "meta":
		err = cmdMeta(os.Args[2:])
	case "sums":
		err = cmdSums(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `apgbuild — APG package builder

Commands:
  build <dir> -o <out.apg>
  meta [-o metadata.json] [--split libs|bins|dev --base-name <name> --version <ver> --arch <arch>] [--detect-deps <dir>]
  sums <dir> <output>`)
}

// cmdBuild: apgbuild build <dir> -o <out.apg> [--compression zstd] [--level 19]
func cmdBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	output := fs.String("o", "", "Output .apg file path (required)")
	compression := fs.String("compression", "zstd", "Compression type: zstd|xz|bz2|gz|lz4|lzma")
	level := fs.Int("level", 0, "Compression level (0 = algorithm default)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: apgbuild build <dir> -o <out.apg>")
	}
	if *output == "" {
		return fmt.Errorf("-o output path is required")
	}
	b := builder.New()
	return b.CreatePackageWithCompression(fs.Arg(0), *output, *compression, *level)
}

// cmdSums: apgbuild sums <dir> <output>
func cmdSums(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: apgbuild sums <dir> <output>")
	}
	_, err := checksum.CreateSums(args[0], args[1])
	return err
}

// cmdMeta: apgbuild meta [flags]
//
// Without --split: runs interactive wizard.
// With --split: generates metadata for a split sub-package.
func cmdMeta(args []string) error {
	fs := flag.NewFlagSet("meta", flag.ContinueOnError)
	output := fs.String("o", "metadata.json", "Output metadata.json path")
	splitKind := fs.String("split", "", "Split kind: libs | bins | dev")
	baseName := fs.String("base-name", "", "Base package name (e.g. curl)")
	version := fs.String("version", "", "Package version")
	arch := fs.String("arch", "", "Architecture (e.g. x86_64)")
	detectDeps := fs.String("detect-deps", "", "Directory to scan for ELF dependencies")
	description := fs.String("description", "", "Package description")
	maintainer := fs.String("maintainer", "", "Package maintainer")
	license := fs.String("license", "", "Package license")
	homepage := fs.String("homepage", "", "Package homepage URL")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *splitKind == "" {
		// Interactive wizard mode
		b := builder.New()
		return b.CreateMetadata(*output)
	}

	// Split metadata generation
	if *baseName == "" {
		return fmt.Errorf("--base-name is required with --split")
	}

	// Build a base Metadata from provided flags (minimal)
	archPtr := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	licensePtr := archPtr(*license) // reuse helper — same *string logic
	base := &metadata.Metadata{
		Version:      *version,
		Architecture: archPtr(*arch),
		Description:  *description,
		Maintainer:   *maintainer,
		License:      licensePtr,
		Homepage:     *homepage,
		Tags:         []string{},
		Conflicts:    []string{},
		Provides:     []string{},
		Replaces:     []string{},
		Conf:         []string{},
	}

	var kind metadata.SplitKind
	switch *splitKind {
	case "libs":
		kind = metadata.SplitLibs
	case "bins":
		kind = metadata.SplitBins
	case "dev":
		kind = metadata.SplitDev
	default:
		return fmt.Errorf("unknown --split value %q: must be libs, bins, or dev", *splitKind)
	}

	splitDir := *detectDeps
	if splitDir == "" {
		// Default to directory of output file
		splitDir = "."
	}

	m, err := metadata.GenerateSplitMetadata(base, splitDir, kind, *baseName)
	if err != nil {
		return fmt.Errorf("generate split metadata: %w", err)
	}

	return m.Save(*output)
}
