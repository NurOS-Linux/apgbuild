use std::fs;
use std::io;
use std::path::{Path, PathBuf};
use std::process::Command;

use sha2::{Digest, Sha256};

use crate::error::{self, ApgError, Result};
use crate::pkgbuild::PkgbuildInfo;

const EXCLUDED_FROM_SRC_COPY: &[&str] = &["APGBUILD", "PKGBUILD", "src", "pkg"];

const BUILD_SCRIPT: &str = r#"
set -euo pipefail
STARTDIR="$1"
RECIPE_PATH="$2"
SRCDIR="$3"
PKGDIR="$4"
PKGVER="$5"
PKGREL="$6"
CARCH="${7:-x86_64}"
: "${MAKEFLAGS:=-j$(nproc)}"
: "${NINJAFLAGS:=-j$(nproc)}"
export MAKEFLAGS
export NINJAFLAGS
cd "$STARTDIR"
source "$RECIPE_PATH"
pkgver="$PKGVER"
pkgrel="$PKGREL"
srcdir="$SRCDIR"
pkgdir="$PKGDIR"
startdir="$STARTDIR"
cd "$srcdir"
if declare -f prepare >/dev/null 2>&1; then
  prepare
fi
cd "$srcdir"
if declare -f build >/dev/null 2>&1; then
  build
fi
cd "$srcdir"
if declare -f package >/dev/null 2>&1; then
  package
fi
"#;

pub struct BuildOutput {
    pub pkgdir: PathBuf,
    #[allow(dead_code)]
    pub srcdir: PathBuf,
    _tempdir: tempfile::TempDir,
}

pub fn prepare_srcdir(startdir: &Path, info: &PkgbuildInfo, clean: bool) -> Result<PathBuf> {
    let srcdir = startdir.join("src");

    if clean && srcdir.exists() {
        remove_dir_all_writable(&srcdir)?;
    }

    if srcdir.exists() {
        return Ok(srcdir);
    }
    fs::create_dir_all(&srcdir).map_err(|e| error::io(&srcdir, e))?;

    for entry in fs::read_dir(startdir).map_err(|e| error::io(startdir, e))? {
        let entry = entry.map_err(|e| error::io(startdir, e))?;
        let file_name = entry.file_name();
        let name = file_name.to_string_lossy();
        if EXCLUDED_FROM_SRC_COPY.iter().any(|n| *n == name) || name.ends_with(".install") {
            continue;
        }
        let dest = srcdir.join(&file_name);
        copy_recursive(&entry.path(), &dest)?;
    }

    extract_local_archives(&srcdir)?;
    resolve_sources(startdir, &srcdir, info)?;

    Ok(srcdir)
}

fn resolve_sources(startdir: &Path, srcdir: &Path, info: &PkgbuildInfo) -> Result<()> {
    for (idx, entry) in info.source.iter().enumerate() {
        let (rename, location) = split_source_entry(entry);
        let location = strip_fragment(&location);

        if is_url(&location) {
            let filename = rename.unwrap_or_else(|| basename_from_url(&location));
            let dest = srcdir.join(&filename);
            if dest.exists() {
                continue;
            }
            let cached = download_cached(&location, &filename)?;
            verify_checksum(info, idx, &filename, &cached)?;
            fs::copy(&cached, &dest).map_err(|e| error::io(&dest, e))?;
            if archive_kind(&filename).is_some() {
                extract_archive(&dest, srcdir)?;
            }
        } else {
            let filename = rename.unwrap_or_else(|| location.clone());
            let dest = srcdir.join(&filename);
            if dest.exists() {
                continue;
            }
            if let Some(found) = find_local_file(startdir, &filename) {
                copy_recursive(&found, &dest)?;
                verify_checksum(info, idx, &filename, &dest)?;
                if archive_kind(&filename).is_some() {
                    extract_archive(&dest, srcdir)?;
                }
            }
        }
    }
    Ok(())
}

fn split_source_entry(entry: &str) -> (Option<String>, String) {
    match entry.split_once("::") {
        Some((name, location)) => (Some(name.to_string()), location.to_string()),
        None => (None, entry.to_string()),
    }
}

fn strip_fragment(location: &str) -> String {
    match location.split_once('#') {
        Some((base, _)) => base.to_string(),
        None => location.to_string(),
    }
}

fn is_url(location: &str) -> bool {
    location.starts_with("http://")
        || location.starts_with("https://")
        || location.starts_with("ftp://")
}

fn basename_from_url(url: &str) -> String {
    url.rsplit('/').next().unwrap_or(url).to_string()
}

fn find_local_file(startdir: &Path, filename: &str) -> Option<PathBuf> {
    let direct = startdir.join(filename);
    if direct.is_file() {
        return Some(direct);
    }
    let shared = startdir.parent()?.join("files").join(filename);
    if shared.is_file() {
        return Some(shared);
    }
    None
}

fn source_cache_dir() -> PathBuf {
    if let Ok(xdg) = std::env::var("XDG_CACHE_HOME") {
        return PathBuf::from(xdg).join("apg").join("sources");
    }
    let home = std::env::var("HOME").unwrap_or_else(|_| ".".to_string());
    PathBuf::from(home)
        .join(".cache")
        .join("apg")
        .join("sources")
}

fn download_cached(url: &str, filename: &str) -> Result<PathBuf> {
    let cache_dir = source_cache_dir();
    fs::create_dir_all(&cache_dir).map_err(|e| error::io(&cache_dir, e))?;

    let cached_path = cache_dir.join(filename);
    if cached_path.is_file() {
        return Ok(cached_path);
    }

    println!("apgbuild: downloading {}", url);

    let response = ureq::get(url)
        .call()
        .map_err(|e| ApgError::Download(url.to_string(), e.to_string()))?;
    let status = response.status();
    if !status.is_success() {
        return Err(ApgError::Download(
            url.to_string(),
            format!("HTTP {}", status),
        ));
    }

    let tmp_path = cache_dir.join(format!("{}.part", filename));
    {
        let mut file = fs::File::create(&tmp_path).map_err(|e| error::io(&tmp_path, e))?;
        let body = response.into_body();
        let mut reader = body.into_reader();
        io::copy(&mut reader, &mut file).map_err(|e| error::io(&tmp_path, e))?;
    }
    fs::rename(&tmp_path, &cached_path).map_err(|e| error::io(&cached_path, e))?;

    Ok(cached_path)
}

fn verify_checksum(info: &PkgbuildInfo, idx: usize, name: &str, file: &Path) -> Result<()> {
    let expected = match info.sha256sums.get(idx) {
        Some(sum) if sum != "SKIP" => sum,
        _ => return Ok(()),
    };

    let actual = sha256_hex(file)?;
    if actual.eq_ignore_ascii_case(expected) {
        Ok(())
    } else {
        Err(ApgError::ChecksumMismatch {
            name: name.to_string(),
            expected: expected.clone(),
            actual,
        })
    }
}

fn sha256_hex(path: &Path) -> Result<String> {
    use std::io::Read;

    let mut file = fs::File::open(path).map_err(|e| error::io(path, e))?;
    let mut hasher = Sha256::new();
    let mut buffer = [0u8; 65536];
    loop {
        let read = file.read(&mut buffer).map_err(|e| error::io(path, e))?;
        if read == 0 {
            break;
        }
        hasher.update(&buffer[..read]);
    }
    let digest = hasher.finalize();
    let mut hex = String::with_capacity(digest.len() * 2);
    for byte in digest {
        hex.push_str(&format!("{:02x}", byte));
    }
    Ok(hex)
}

fn extract_local_archives(srcdir: &Path) -> Result<()> {
    for entry in fs::read_dir(srcdir).map_err(|e| error::io(srcdir, e))? {
        let entry = entry.map_err(|e| error::io(srcdir, e))?;
        let path = entry.path();
        if !path.is_file() {
            continue;
        }
        let name = entry.file_name();
        let name = name.to_string_lossy();
        if archive_kind(&name).is_some() {
            extract_archive(&path, srcdir)?;
        }
    }
    Ok(())
}

fn archive_kind(name: &str) -> Option<&'static str> {
    let suffixes: &[(&str, &str)] = &[
        (".tar.gz", "tar"),
        (".tgz", "tar"),
        (".tar.xz", "tar"),
        (".txz", "tar"),
        (".tar.bz2", "tar"),
        (".tbz2", "tar"),
        (".tar.zst", "tar"),
        (".tar", "tar"),
        (".zip", "zip"),
    ];
    suffixes
        .iter()
        .find(|(suffix, _)| name.ends_with(suffix))
        .map(|(_, kind)| *kind)
}

fn extract_archive(archive: &Path, srcdir: &Path) -> Result<()> {
    let name = archive
        .file_name()
        .unwrap_or_default()
        .to_string_lossy()
        .into_owned();
    let status = match archive_kind(&name) {
        Some("tar") => Command::new("tar")
            .arg("xf")
            .arg(archive)
            .arg("-C")
            .arg(srcdir)
            .status(),
        Some("zip") => Command::new("unzip")
            .arg("-o")
            .arg(archive)
            .arg("-d")
            .arg(srcdir)
            .status(),
        _ => return Ok(()),
    }
    .map_err(ApgError::BashSpawn)?;

    if !status.success() {
        return Err(ApgError::BashFailed {
            status: status.to_string(),
            stderr: format!(
                "failed to extract local source archive {}",
                archive.display()
            ),
        });
    }
    Ok(())
}

fn copy_recursive(src: &Path, dest: &Path) -> Result<()> {
    if src.is_dir() {
        fs::create_dir_all(dest).map_err(|e| error::io(dest, e))?;
        for entry in fs::read_dir(src).map_err(|e| error::io(src, e))? {
            let entry = entry.map_err(|e| error::io(src, e))?;
            copy_recursive(&entry.path(), &dest.join(entry.file_name()))?;
        }
    } else {
        fs::copy(src, dest).map_err(|e| error::io(src, e))?;
    }
    Ok(())
}

fn remove_dir_all_writable(path: &Path) -> Result<()> {
    make_writable_recursive(path)?;
    fs::remove_dir_all(path).map_err(|e| error::io(path, e))
}

fn make_writable_recursive(path: &Path) -> Result<()> {
    let meta = fs::symlink_metadata(path).map_err(|e| error::io(path, e))?;
    if meta.file_type().is_symlink() {
        return Ok(());
    }
    set_writable(path, &meta)?;
    if meta.is_dir() {
        for entry in fs::read_dir(path).map_err(|e| error::io(path, e))? {
            let entry = entry.map_err(|e| error::io(path, e))?;
            make_writable_recursive(&entry.path())?;
        }
    }
    Ok(())
}

#[cfg(unix)]
fn set_writable(path: &Path, meta: &fs::Metadata) -> Result<()> {
    use std::os::unix::fs::PermissionsExt;
    let mut perms = meta.permissions();
    let mode = perms.mode() | 0o700;
    perms.set_mode(mode);
    fs::set_permissions(path, perms).map_err(|e| error::io(path, e))
}

#[cfg(not(unix))]
fn set_writable(path: &Path, meta: &fs::Metadata) -> Result<()> {
    let mut perms = meta.permissions();
    perms.set_readonly(false);
    fs::set_permissions(path, perms).map_err(|e| error::io(path, e))
}

#[allow(clippy::too_many_arguments)]
pub fn run_build_and_package(
    startdir: &Path,
    recipe_path: &Path,
    srcdir: &Path,
    info: &PkgbuildInfo,
    arch: &str,
    allow_empty: bool,
) -> Result<BuildOutput> {
    let tempdir = tempfile::Builder::new()
        .prefix("apgbuild-pkgdir-")
        .tempdir()
        .map_err(|e| error::io(std::env::temp_dir(), e))?;
    let pkgdir = tempdir.path().join("pkg");
    fs::create_dir_all(&pkgdir).map_err(|e| error::io(&pkgdir, e))?;

    crate::pkgbuild::bash_run(
        BUILD_SCRIPT,
        &[
            startdir.to_str().unwrap_or("."),
            recipe_path.to_str().unwrap_or("PKGBUILD"),
            srcdir.to_str().unwrap_or("."),
            pkgdir.to_str().unwrap_or("."),
            &info.pkgver,
            &info.pkgrel,
            arch,
        ],
        startdir,
    )?;

    let has_entries = fs::read_dir(&pkgdir)
        .map_err(|e| error::io(&pkgdir, e))?
        .next()
        .is_some();
    if !has_entries && !allow_empty {
        return Err(ApgError::EmptyPkgdir);
    }

    Ok(BuildOutput {
        pkgdir,
        srcdir: srcdir.to_path_buf(),
        _tempdir: tempdir,
    })
}
