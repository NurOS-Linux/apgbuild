// Package elfanalyzer extracts shared library dependencies from ELF binaries.
// Uses debug/elf standard library — no external dependencies.
// NurOS 2026 - GPL 3.0
package elfanalyzer

import (
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LibInfo describes a shared library dependency.
type LibInfo struct {
	Name     string // e.g. "libc.so.6"
	NeededBy string // the binary that needs it
}

// ExtractDependencies reads an ELF binary and returns its NEEDED shared libraries.
func ExtractDependencies(binaryPath string) ([]LibInfo, error) {
	f, err := elf.Open(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("open elf binary %s: %w", binaryPath, err)
	}
	defer f.Close()

	// Only ELF executables and shared objects have dynamic dependencies
	if f.Type != elf.ET_EXEC && f.Type != elf.ET_DYN {
		return nil, nil
	}

	imported, err := f.ImportedLibraries()
	if err != nil {
		return nil, fmt.Errorf("read imported libraries: %w", err)
	}

	libs := make([]LibInfo, 0, len(imported))
	for _, lib := range imported {
		libs = append(libs, LibInfo{
			Name:     lib,
			NeededBy: binaryPath,
		})
	}

	return libs, nil
}

// ExtractFromDir scans a directory tree for ELF binaries and extracts
// all shared library dependencies.
func ExtractFromDir(dir string) ([]LibInfo, error) {
	var allLibs []LibInfo

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Skip non-executable files quickly
		if info.Mode()&0111 == 0 {
			return nil
		}

		libs, err := ExtractDependencies(path)
		if err != nil {
			// Not an ELF binary — skip silently
			return nil
		}

		allLibs = append(allLibs, libs...)
		return nil
	})

	return allLibs, err
}

// LibToPackageMap maps common library names to Fedora package names.
var LibToPackageMap = map[string]string{
	// glibc
	"libc.so.6":       "glibc",
	"libpthread.so.0": "glibc",
	"libdl.so.2":      "glibc",
	"libm.so.6":       "glibc",
	"librt.so.1":      "glibc",
	"libresolv.so.2":  "glibc",
	"libnsl.so.1":     "libnsl",
	"libutil.so.1":    "glibc",
	"libanl.so.1":     "glibc",

	// compression
	"libz.so.1":           "zlib",
	"libzstd.so.1":        "zstd-libs",
	"liblzma.so.5":        "xz-libs",
	"libbz2.so.1":         "bzip2-libs",
	"liblz4.so.1":         "lz4-libs",
	"libbrotlicommon.so.1": "brotli",
	"libbrotlidec.so.1":   "brotli",
	"libbrotlienc.so.1":   "brotli",

	// crypto
	"libssl.so.3":     "openssl-libs",
	"libcrypto.so.3":  "openssl-libs",

	// c++
	"libstdc++.so.6": "libstdc++",
	"libgcc_s.so.1":  "libgcc",

	// networking
	"libcurl.so.4":       "curl",
	"libnghttp2.so.14":   "nghttp2",
	"libidn2.so.0":       "libidn2",
	"libunistring.so.2":  "libunistring",
	"libpsl.so.5":        "libpsl",
	"libssh.so.4":        "libssh",

	// ldap
	"libldap.so.2":  "openldap",
	"liblber.so.2":  "openldap",

	// terminal
	"libtinfow.so.6":  "ncurses",
	"libncursesw.so.6": "ncurses",
	"libreadline.so.8": "readline",
	"libgpm.so.2":     "gpm",

	// system
	"libpcre2-8.so.0":    "pcre2",
	"libselinux.so.1":    "libselinux",
	"libsepol.so.2":      "libsepol",
	"libpopt.so.0":       "popt",
	"librpmio.so.9":      "rpm-libs",
	"librpm.so.9":        "rpm-libs",
	"libxml2.so.2":       "libxml2",
	"libsqlite3.so.0":    "sqlite-libs",
	"libexpat.so.1":      "expat",
	"libffi.so.8":        "libffi",
	"libcap.so.2":        "libcap",
	"libaudit.so.1":      "audit-libs",
	"libsystemd.so.0":    "systemd-libs",
	"libgcrypt.so.20":    "libgcrypt",
	"libgpg-error.so.0":  "libgpg-error",
	"libtasn1.so.6":      "libtasn1",
	"libp11-kit.so.0":    "p11-kit",
	"libkeyutils.so.1":   "keyutils-libs",
	"libkrb5.so.3":       "krb5-libs",
	"libk5crypto.so.3":   "krb5-libs",
	"libcom_err.so.2":    "libcom_err",
	"libgssapi_krb5.so.2": "krb5-libs",
}

// MapLibsToPackages maps library names to Fedora package names.
func MapLibsToPackages(libs []LibInfo) []string {
	pkgSet := make(map[string]bool)
	for _, lib := range libs {
		if pkg, ok := LibToPackageMap[lib.Name]; ok {
			pkgSet[pkg] = true
		}
	}

	pkgs := make([]string, 0, len(pkgSet))
	for pkg := range pkgSet {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)

	return pkgs
}

// AnalyzeAndGenerateDeps analyzes a directory for ELF binaries and generates
// a dependency list suitable for metadata.json.
func AnalyzeAndGenerateDeps(rootDir string) ([]string, error) {
	libs, err := ExtractFromDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("extract dependencies: %w", err)
	}

	return MapLibsToPackages(libs), nil
}

// GenerateDependenciesJSON formats a dependency list as a JSON array string.
func GenerateDependenciesJSON(deps []string) string {
	if len(deps) == 0 {
		return "[]"
	}

	quoted := make([]string, len(deps))
	for i, dep := range deps {
		quoted[i] = fmt.Sprintf("%q", dep)
	}

	return "[" + strings.Join(quoted, ", ") + "]"
}
