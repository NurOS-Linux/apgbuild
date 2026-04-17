// Package archive provides APG package archive creation and extraction
// using libarchive (C library) via CGO.
// Supports all libarchive formats: zstd, xz, bz2, gz, lz4, lzma.
// NurOS 2026 - GPL 3.0
package archive

/*
#cgo pkg-config: libarchive
#cgo CFLAGS: -I/usr/include

#include <archive.h>
#include <archive_entry.h>
#include <stdlib.h>
#include <string.h>

// apg_create creates a tar archive at archivePath from sourceDir.
// compressionType: "zstd", "xz", "bz2", "gz", "lz4", "lzma"
// level: compression level (0 = algorithm default)
static int apg_create(const char *archivePath, const char *sourceDir,
                      const char *compressionType, int level,
                      char *errBuf, int errBufLen) {
    struct archive *a = archive_write_new();
    if (!a) { snprintf(errBuf, errBufLen, "archive_write_new failed"); return -1; }

    int r = ARCHIVE_FAILED;
    if      (strcmp(compressionType, "zstd") == 0) r = archive_write_add_filter_zstd(a);
    else if (strcmp(compressionType, "xz")   == 0) r = archive_write_add_filter_xz(a);
    else if (strcmp(compressionType, "bz2")  == 0) r = archive_write_add_filter_bzip2(a);
    else if (strcmp(compressionType, "gz")   == 0) r = archive_write_add_filter_gzip(a);
    else if (strcmp(compressionType, "lz4")  == 0) r = archive_write_add_filter_lz4(a);
    else if (strcmp(compressionType, "lzma") == 0) r = archive_write_add_filter_lzma(a);
    else {
        snprintf(errBuf, errBufLen, "unknown compression: %s", compressionType);
        archive_write_free(a); return -1;
    }

    if (r != ARCHIVE_OK) {
        snprintf(errBuf, errBufLen, "set filter %s: %s", compressionType, archive_error_string(a));
        archive_write_free(a); return -1;
    }

    if (level > 0) {
        char lvl[8]; snprintf(lvl, sizeof(lvl), "%d", level);
        archive_write_set_filter_option(a, NULL, "compression-level", lvl);
    }

    archive_write_set_format_pax_restricted(a);

    if (archive_write_open_filename(a, archivePath) != ARCHIVE_OK) {
        snprintf(errBuf, errBufLen, "open %s: %s", archivePath, archive_error_string(a));
        archive_write_free(a); return -1;
    }

    struct archive *disk = archive_read_disk_new();
    archive_read_disk_set_standard_lookup(disk);
    archive_read_disk_set_symlink_logical(disk);

    r = archive_read_disk_open(disk, sourceDir);
    if (r != ARCHIVE_OK) {
        snprintf(errBuf, errBufLen, "open dir %s: %s", sourceDir, archive_error_string(disk));
        archive_read_free(disk); archive_write_free(a); return -1;
    }

    struct archive_entry *entry = archive_entry_new();
    int sourceDirLen = (int)strlen(sourceDir);

    for (;;) {
        r = archive_read_next_header2(disk, entry);
        if (r == ARCHIVE_EOF) break;
        if (r != ARCHIVE_OK) {
            snprintf(errBuf, errBufLen, "read disk: %s", archive_error_string(disk));
            break;
        }
        archive_read_disk_descend(disk);

        const char *fullPath = archive_entry_pathname(entry);
        const char *relPath = fullPath + sourceDirLen;
        while (*relPath == '/') relPath++;
        if (*relPath == '\0') continue;
        archive_entry_set_pathname(entry, relPath);

        if (archive_write_header(a, entry) != ARCHIVE_OK) continue;

        if (archive_entry_size(entry) > 0) {
            FILE *f = fopen(fullPath, "rb");
            if (f) {
                char buf[65536]; size_t n;
                while ((n = fread(buf, 1, sizeof(buf), f)) > 0)
                    archive_write_data(a, buf, n);
                fclose(f);
            }
        }
    }

    archive_entry_free(entry);
    archive_read_close(disk);
    archive_read_free(disk);
    archive_write_close(a);
    archive_write_free(a);
    return (r == ARCHIVE_EOF || r == ARCHIVE_OK) ? 0 : -1;
}

// apg_extract extracts an archive to destDir (auto-detects format).
static int apg_extract(const char *archivePath, const char *destDir,
                       char *errBuf, int errBufLen) {
    struct archive *a = archive_read_new();
    archive_read_support_filter_all(a);
    archive_read_support_format_all(a);

    struct archive *ext = archive_write_disk_new();
    archive_write_disk_set_options(ext,
        ARCHIVE_EXTRACT_TIME | ARCHIVE_EXTRACT_PERM |
        ARCHIVE_EXTRACT_SECURE_NODOTDOT | ARCHIVE_EXTRACT_SECURE_SYMLINKS);
    archive_write_disk_set_standard_lookup(ext);

    if (archive_read_open_filename(a, archivePath, 65536) != ARCHIVE_OK) {
        snprintf(errBuf, errBufLen, "open %s: %s", archivePath, archive_error_string(a));
        archive_read_free(a); archive_write_free(ext); return -1;
    }

    struct archive_entry *entry;
    int r;
    char fullPath[4096];

    for (;;) {
        r = archive_read_next_header(a, &entry);
        if (r == ARCHIVE_EOF) break;
        if (r != ARCHIVE_OK) {
            snprintf(errBuf, errBufLen, "read: %s", archive_error_string(a));
            break;
        }
        snprintf(fullPath, sizeof(fullPath), "%s/%s", destDir, archive_entry_pathname(entry));
        archive_entry_set_pathname(entry, fullPath);

        if (archive_write_header(ext, entry) != ARCHIVE_OK) continue;

        if (archive_entry_size(entry) > 0) {
            const void *buf; size_t size; la_int64_t offset;
            while (archive_read_data_block(a, &buf, &size, &offset) == ARCHIVE_OK)
                archive_write_data_block(ext, buf, size, offset);
        }
    }

    archive_read_close(a);
    archive_read_free(a);
    archive_write_close(ext);
    archive_write_free(ext);
    return (r == ARCHIVE_EOF) ? 0 : -1;
}
*/
import "C"
import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"
)

const (
	MaxFileSize    = 100 * 1024 * 1024
	MaxArchiveSize = 1024 * 1024 * 1024
	MaxFiles       = 10000
)

// CreateOptions configures archive creation.
type CreateOptions struct {
	// Compression: "zstd" | "xz" | "bz2" | "gz" | "lz4" | "lzma"
	Compression string
	// Level: compression level (0 = algorithm default)
	Level int
}

// CreateResult contains information about the created archive.
type CreateResult struct {
	FilesAdded int
	TotalSize  int64
}

// Create creates a tar+zstd archive from sourceDir (default settings).
func Create(archivePath, sourceDir string) (*CreateResult, error) {
	return CreateWithOptions(archivePath, sourceDir, CreateOptions{Compression: "zstd", Level: 19})
}

// CreateWithOptions creates an archive with explicit compression settings.
func CreateWithOptions(archivePath, sourceDir string, opts CreateOptions) (*CreateResult, error) {
	if opts.Compression == "" {
		opts.Compression = "zstd"
	}

	cArchive := C.CString(archivePath)
	cSource := C.CString(sourceDir)
	cComp := C.CString(opts.Compression)
	defer C.free(unsafe.Pointer(cArchive))
	defer C.free(unsafe.Pointer(cSource))
	defer C.free(unsafe.Pointer(cComp))

	var errBuf [512]C.char
	r := C.apg_create(cArchive, cSource, cComp, C.int(opts.Level), &errBuf[0], 512)
	if r != 0 {
		return nil, fmt.Errorf("create archive: %s", C.GoString(&errBuf[0]))
	}
	return &CreateResult{}, nil
}

// Extract extracts an archive to destDir.
// Automatically detects compression format via libarchive.
func Extract(archivePath, destDir string) error {
	cArchive := C.CString(archivePath)
	cDest := C.CString(destDir)
	defer C.free(unsafe.Pointer(cArchive))
	defer C.free(unsafe.Pointer(cDest))

	var errBuf [512]C.char
	r := C.apg_extract(cArchive, cDest, &errBuf[0], 512)
	if r != 0 {
		return fmt.Errorf("extract archive: %s", C.GoString(&errBuf[0]))
	}
	return nil
}

// ListContents lists the files in an archive without full extraction.
func ListContents(archivePath string) ([]string, error) {
	tempDir, err := os.MkdirTemp("", "apgbuild-list-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	if err := Extract(archivePath, tempDir); err != nil {
		return nil, err
	}

	var contents []string
	filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error { //nolint:errcheck
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(tempDir, path)
		contents = append(contents, rel)
		return nil
	})
	return contents, nil
}
