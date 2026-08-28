# apgbuild

Converts Arch Linux PKGBUILD packages into APGv2 packages for NurOS (Tulpar).

> [!NOTE]
> **Repository Mirrors**
>
> - **git.nuros.org** ([utils/apgbuild](https://git.nuros.org/utils/apgbuild)): primary, self-hosted Forgejo instance, accounts restricted to the core team.
> - **GitHub** ([NurOS-Linux/apgbuild](https://github.com/NurOS-Linux/apgbuild)): mirror for external contributors. Issues and Pull Requests opened here are welcome and are reviewed and processed by the core team.

## What it does

`apgbuild` takes a directory containing a PKGBUILD (and its accompanying install script and local source files), runs the real `prepare()`/`build()`/`package()` shell functions the same way `makepkg` would, and repackages the resulting `$pkgdir` tree as an APGv2 `.apg` archive with a generated `metadata.json`.

PKGBUILD variables are read through bash's own `declare -p` serialization rather than a hand-rolled shell grammar, so array/string handling matches what bash itself produces. Install-script hooks (`pre_install`, `post_install`, `pre_remove`, `post_remove` from the file referenced by `install=`) are extracted with `declare -f` and turned into standalone executable scripts under `scripts/`.

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
- `--arch <x86_64|aarch64|riscv64>` - target architecture; defaults to the host architecture. Must be listed in the PKGBUILD's `arch=(...)` array, unless the PKGBUILD declares `arch=('any')`, in which case `architecture` in metadata.json is set to `"all"` regardless of this flag.
- `--type <binary|source|misc>` - APGv2 package type (default `binary`).
- `--maintainer "Name <email>"` - overrides the `# Maintainer:` comment auto-detected from the PKGBUILD header.
- `--tag foo --tag bar` (or `--tag foo,bar`) - tags for metadata.json (PKGBUILD has no native concept of tags).
- `--sign-key mykey.key` - sign the resulting `.apg` file (see below).

Source files referenced by the PKGBUILD are expected to already be present next to it (no network fetching is performed). If the directory has no `src/` subdirectory yet, apgbuild copies everything except `PKGBUILD` and `*.install` files into a freshly created `src/`, then extracts any recognized local archives found there (`.tar.gz`/`.tgz`, `.tar.xz`/`.txz`, `.tar.bz2`/`.tbz2`, `.tar.zst`, `.tar`, `.zip`) in place, mirroring what makepkg does with downloaded sources, before invoking `prepare()`/`build()`/`package()`.

Because `package()` always installs into `$pkgdir` (a throwaway directory under a fresh `tempfile` temporary directory, never the real filesystem), running `apgbuild build` never writes to the host's real `/usr`, `/etc`, and so on, even when the PKGBUILD's `package()` uses `install -Dm... "$pkgdir/usr/bin/..."` verbatim, exactly as makepkg guarantees.

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
