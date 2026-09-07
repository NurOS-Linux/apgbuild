# apgbuild

Converts Arch Linux PKGBUILD packages into APGv2 packages for NurOS (Tulpar).

> [!NOTE]
> **Repository Mirrors**
>
> - **git.nuros.org** ([utils/apgbuild](https://git.nuros.org/utils/apgbuild)): primary, self-hosted Forgejo instance, accounts restricted to the core team.
> - **GitHub** ([NurOS-Linux/apgbuild](https://github.com/NurOS-Linux/apgbuild)): mirror for external contributors. Issues and Pull Requests opened here are welcome and are reviewed and processed by the core team.

## What it does

`apgbuild` takes a directory containing an `APGBUILD` or `PKGBUILD` recipe (and its accompanying install script and local source files), runs the real `prepare()`/`build()`/`package()` shell functions the same way `makepkg` would, and repackages the resulting `$pkgdir` tree as an APGv2 `.apg` archive with a generated `metadata.json`.

Recipe variables are read through bash's own `declare -p` serialization rather than a hand-rolled shell grammar, so array/string handling matches what bash itself produces. Install hooks (`pre_install`, `post_install`, `pre_remove`, `post_remove`) can be declared directly as functions in the recipe, or in an external `.install` file referenced via `install=`; both are extracted with `declare -f` and turned into standalone executable scripts under `scripts/`.

## APGBUILD: the extended recipe format

`APGBUILD` is a drop-in superset of `PKGBUILD`: same bash syntax, same `build()`/`package()` convention, plus a handful of variables and hooks that are native to NurOS instead of being bolted on through CLI flags or external `.install` files. `apgbuild build <dir>` looks for `APGBUILD` first and falls back to `PKGBUILD` if it isn't present, so existing Arch recipes work unmodified.

Native additions on top of PKGBUILD:

- `pkgtype=binary|source|misc` - equivalent to `--type`; a CLI `--type` still overrides it.
- `maintainer="Name <email>"` - equivalent to `--maintainer`; falls back to the `# Maintainer:` header comment, then to `"Unknown"`.
- `tags=('base' 'system')` - equivalent to `--tag`; a non-empty `--tag` list overrides it.
- `conf=('/etc/foo.conf')` - protected config paths, unioned with Arch's `backup=()` in the final `conf` field (deduplicated, both normalized to start with `/`).
- `pre_install()`, `post_install()`, `pre_remove()`, `post_remove()` declared directly in the recipe - packaged the same way as an external `.install` file's hooks, and take priority over it if both define the same hook.

`pkgtype=misc` (or a recipe with no `package()` function and no `source=()`, such as a pure metapackage) is exempt from the "package() must produce files" check: an empty `$pkgdir` is valid, and the archive still gets a `data/` directory entry (required by `libapg`'s installer, which refuses to install an archive that lacks one) even when it is empty.

## Building

```
cargo build --release
```

Requires a `bash` binary on `PATH` (used to source and execute the PKGBUILD, exactly like makepkg does) and a C toolchain for the `zstd`/`xz` compression backends.

## Usage

```
apgbuild build ./path/to/pkgbuild-dir -o output.apg --compression zst
```

Useful flags:

- `--compression xz|zst` - compression backend for the final archive (default `zst`).
- `--arch <x86_64|aarch64|riscv64>` - target architecture; defaults to the host architecture. Must be listed in the recipe's `arch=(...)` array, unless it declares `arch=('any')`, in which case `architecture` in metadata.json is set to `"all"` regardless of this flag.
- `--type <binary|source|misc>` - APGv2 package type; overrides the recipe's native `pkgtype=`, defaults to `binary` if neither is set.
- `--maintainer "Name <email>"` - overrides the recipe's native `maintainer=` and the `# Maintainer:` header comment fallback.
- `--tag foo --tag bar` (or `--tag foo,bar`) - overrides the recipe's native `tags=(...)`.
- `-o, --output <path>` - exact output file path, or an existing directory to place the auto-named file into.
- `--output-dir <dir>` - directory to place the auto-named `.apg` file into; created if missing. Takes effect when `--output` isn't given, or when `--output` points at a path that doesn't exist yet as a directory.
- `-c, --clean` - wipe an existing `src/` before repreparing it, instead of reusing whatever a previous (possibly failed) run left behind.
- `--sign-key mykey.key` - sign the resulting `.apg` file (see below).

### Sources: local files, downloads, and caching

Every entry in `source=()` is resolved the same way makepkg resolves it, minus VCS sources (`git+`, `svn+`, and similar prefixes are not supported):

- **`name::url`** renames the fetched/copied file to `name`.
- **`http://`, `https://`, `ftp://` URLs** are downloaded with `ureq` into a shared cache (`$XDG_CACHE_HOME/apg/sources`, or `~/.cache/apg/sources` if that's unset) keyed by filename, then copied into `src/`. A cache hit skips the network entirely, so repeated builds of the same version don't redownload anything.
- **Everything else** is treated as a local filename and looked up first directly next to the recipe (`startdir/<name>`), then in a sibling `../files/<name>` directory, so patches and configs shared across multiple recipes don't need `local files_dir="${startdir}/../files"` boilerplate.
- **`sha256sums=()`** is checked index-for-index against `source=()`; `'SKIP'` (or a missing entry) skips verification for that source, anything else must match exactly or the build fails before `prepare()`/`build()`/`package()` ever run.

Recognized local archives (`.tar.gz`/`.tgz`, `.tar.xz`/`.txz`, `.tar.bz2`/`.tbz2`, `.tar.zst`, `.tar`, `.zip`), whether they arrived via `source=()` or were just sitting next to the recipe, are extracted into `src/` in place, mirroring what makepkg does after fetching sources.

`src/` is only prepared once and then reused by later runs (so an interrupted build can be resumed without redownloading); pass `--clean` to force a full re-prepare. `--clean` recursively makes everything writable before removing it, so read-only trees left behind by Go, Cargo, or Git (which mark files, and sometimes whole directories such as Go's module cache, as read-only) don't cause a `Permission denied`.

`prepare()`, `build()`, and `package()` run with `MAKEFLAGS` and `NINJAFLAGS` defaulted to `-j$(nproc)` if the recipe (or the environment `apgbuild` was invoked from) doesn't already set them, so a clean-room build isn't accidentally single-threaded.

Because `package()` always installs into `$pkgdir` (a throwaway directory under a fresh `tempfile` temporary directory, never the real filesystem), running `apgbuild build` never writes to the host's real `/usr`, `/etc`, and so on, even when the recipe's `package()` uses `install -Dm... "$pkgdir/usr/bin/..."` verbatim, exactly as makepkg guarantees.

### Symlinks

`$pkgdir` trees commonly contain symlinks that don't resolve inside the build sandbox: relative ones like `libfoo.so -> libfoo.so.1` are fine once installed but may or may not point at something real inside `$pkgdir` depending on build order, and absolute ones like `python3 -> /usr/bin/python3.11` are never meant to resolve inside `$pkgdir` at all. `apgbuild` archives symlinks as symlinks (`tar`'s `follow_symlinks(false)`) instead of trying to open and read through them, so packages with library symlinks or other absolute/dangling links archive correctly instead of failing with an "No such file or directory" I/O error.

### Key generation

```
apgbuild keygen -o mykey
```

Writes `mykey.pub.key` (raw 32-byte Ed25519 public key) and `mykey.secret` (raw 64-byte Ed25519 secret key, written with `0600` permissions). Keep `mykey.secret` private; `mykey.pub.key` is meant to be distributed to anyone who needs to verify packages signed with it. Both files hold the exact bytes libsodium's `crypto_sign_keypair` produces, no text encoding of any kind.

### Verification

```
apgbuild verify output.apg --pubkey output.apg.pub.key
```

By default the signature file is expected at `<package>.sig`; override with `--signature`.

## Signature scheme

APGv2 packages carry no embedded checksums or signatures inside the archive itself. Instead, `apgbuild build --sign-key` signs the finished, compressed `.apg` file as a whole with Ed25519 (via `dryoc`, a pure-Rust libsodium-compatible implementation) and writes two files next to it:

- `<output>.apg.sig` - the raw 64-byte detached signature. Not text-encoded.
- `<output>.apg.pub.key` - the raw 32-byte Ed25519 public key, derived from the secret key used to sign. Not text-encoded; the `.key` suffix lets it be dropped directly into a NurOS trusted keyring directory (or passed to `tulpar key add`) without renaming.

All key and signature files are exactly the bytes libsodium's `crypto_sign` API reads and writes with `fread`/`fwrite` - deliberately not hex, base64, or PEM - because this format has to interoperate byte-for-byte with `libapg` (`src/sign/sodium/sodium.c`, `src/sign/sodium/keyring.c`) and Tulpar's `key add`/install-time verification, which both do raw binary reads of fixed sizes (`crypto_sign_BYTES` = 64, `crypto_sign_PUBLICKEYBYTES` = 32, `crypto_sign_SECRETKEYBYTES` = 64) with no framing or encoding. `apgbuild`'s signing uses dryoc's incremental signer (`crypto_sign_init`/`update`/`final_create`/`final_verify`), which implements the same Ed25519ph construction as libsodium's incremental API that `libapg` calls, so signatures produced by `apgbuild` verify correctly against the real `libapg`/Tulpar keyring and vice versa - this has been checked directly against `libapg`'s C implementation, not just inferred from matching byte sizes.

This keeps the package archive itself untouched by the signing step (so rebuilding the archive is deterministic and reproducible independent of key material) and keeps key distribution simple: a repository or a NurOS install medium ships trusted `*.key` files, and `apgbuild verify` (or Tulpar's own install-time `keyring_verify`) only needs the `.apg`, the `.sig`, and a trusted public key to confirm the archive was produced by the holder of the matching secret key and has not been modified since.

To verify a package outside of `apgbuild verify`, any Ed25519 implementation that supports the incremental/prehashed (Ed25519ph) signing mode can be used directly against the raw bytes of the `.pub.key`, `.sig`, and `.apg` files - no decoding step is needed first.
