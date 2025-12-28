# APGBuild

Package build system for NurOS Linux.

## About

APGBuild is a tool for creating and managing APGv2 packages. It provides CRC32 checksums for file integrity verification and secure archive handling.

## Features

- APGv2 package creation and extraction
- CRC32 checksums (replaces deprecated MD5)
- Interactive metadata.json wizard
- Path traversal protection
- tar.xz compression

## Requirements

- Go 1.21 or later
- Meson (for building)
- Ninja

## Building

### Using Meson

```bash
meson setup build
meson compile -C build
sudo meson install -C build
```

### Using Go

```bash
go build -o apgbuild ./cmd/apgbuild
```

## Usage

```
apgbuild [command] [options]

Commands:
  build, -b <dir> [-o <output>]  Build package from directory
  extract, -x <pkg> [dest]       Extract package
  list, -l <pkg>                 List package contents
  meta, -m [output]              Create metadata.json
  sums <dir> [output]            Generate CRC32 checksums
  verify <sums> [basedir]        Verify checksums
  version, -v                    Show version
  help, -h                       Show help
```

### Examples

```bash
# Build package
apgbuild build ./mypackage -o mypackage.apg

# Extract package
apgbuild extract package.apg ./output

# Create metadata
apgbuild meta

# Generate checksums
apgbuild sums ./data crc32sums
```

## Package Structure

```
package.apg
├── metadata.json
├── crc32sums
├── data/
├── scripts/
│   ├── pre-install
│   └── post-install
└── home/
```

## License

GPL-3.0

## Author

AnmiTaliDev <anmitali198@gmail.com>
