use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

use crate::error::{self, ApgError, Result};
use crate::pkgbuild::PkgbuildInfo;

const BUILD_SCRIPT: &str = r#"
set -euo pipefail
STARTDIR="$1"
PKGBUILD_PATH="$2"
SRCDIR="$3"
PKGDIR="$4"
PKGVER="$5"
PKGREL="$6"
CARCH="${7:-x86_64}"
cd "$STARTDIR"
source "$PKGBUILD_PATH"
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

pub fn prepare_srcdir(startdir: &Path) -> Result<PathBuf> {
    let srcdir = startdir.join("src");
    if srcdir.exists() {
        return Ok(srcdir);
    }
    fs::create_dir_all(&srcdir).map_err(|e| error::io(&srcdir, e))?;

    for entry in fs::read_dir(startdir).map_err(|e| error::io(startdir, e))? {
        let entry = entry.map_err(|e| error::io(startdir, e))?;
        let file_name = entry.file_name();
        let name = file_name.to_string_lossy();
        if name == "PKGBUILD" || name == "src" || name == "pkg" || name.ends_with(".install") {
            continue;
        }
        let dest = srcdir.join(&file_name);
        copy_recursive(&entry.path(), &dest)?;
    }

    extract_local_archives(&srcdir)?;

    Ok(srcdir)
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
    let name = archive.file_name().unwrap_or_default().to_string_lossy().into_owned();
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
            stderr: format!("failed to extract local source archive {}", archive.display()),
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

pub fn run_build_and_package(
    startdir: &Path,
    srcdir: &Path,
    info: &PkgbuildInfo,
    arch: &str,
) -> Result<BuildOutput> {
    let tempdir = tempfile::Builder::new()
        .prefix("apgbuild-pkgdir-")
        .tempdir()
        .map_err(|e| error::io(std::env::temp_dir(), e))?;
    let pkgdir = tempdir.path().join("pkg");
    fs::create_dir_all(&pkgdir).map_err(|e| error::io(&pkgdir, e))?;

    let pkgbuild_path = startdir.join("PKGBUILD");

    crate::pkgbuild::bash_run(
        BUILD_SCRIPT,
        &[
            startdir.to_str().unwrap_or("."),
            pkgbuild_path.to_str().unwrap_or("PKGBUILD"),
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
    if !has_entries {
        return Err(ApgError::EmptyPkgdir);
    }

    Ok(BuildOutput {
        pkgdir,
        srcdir: srcdir.to_path_buf(),
        _tempdir: tempdir,
    })
}
