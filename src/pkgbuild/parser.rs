use std::fs;
use std::path::Path;

use super::bash::{parse_declare_block, run_bash_script, DeclareValue};
use super::model::{InstallHooks, PkgbuildInfo};
use crate::error::{ApgError, Result};

const VARS_MARKER: &str = "###APGBUILD:VARS###";
const FUNCS_MARKER: &str = "###APGBUILD:FUNCS###";
const INSTALLFILE_MARKER: &str = "###APGBUILD:INSTALLFILE###";
const HOOK_MARKER_PREFIX: &str = "###APGBUILD:HOOK:";
const HOOK_MARKER_SUFFIX: &str = "###";

const EXTRACT_SCRIPT: &str = r#"
set -euo pipefail
STARTDIR="$1"
PKGBUILD_PATH="$2"
SRCDIR="$3"
CARCH="${4:-x86_64}"
cd "$STARTDIR"
source "$PKGBUILD_PATH"
if declare -f pkgver >/dev/null 2>&1; then
  cd "$SRCDIR"
  pkgver="$(pkgver)"
  cd "$STARTDIR"
fi
echo '###APGBUILD:VARS###'
declare -p pkgname pkgver pkgrel epoch pkgdesc arch url license depends optdepends makedepends conflicts provides replaces backup source install 2>/dev/null || true
echo '###APGBUILD:FUNCS###'
for f in prepare build package; do
  if declare -f "$f" >/dev/null 2>&1; then
    echo "$f"
  fi
done
echo '###APGBUILD:INSTALLFILE###'
if [ -n "${install:-}" ] && [ -f "$STARTDIR/$install" ]; then
  echo "$STARTDIR/$install"
fi
"#;

const INSTALL_HOOKS_SCRIPT: &str = r####"
set -euo pipefail
INSTALL_FILE="$1"
source "$INSTALL_FILE"
for hookname in pre_install post_install pre_remove post_remove; do
  if declare -f "$hookname" >/dev/null 2>&1; then
    echo "###APGBUILD:HOOK:${hookname}###"
    declare -f "$hookname"
  fi
done
"####;

pub fn parse_pkgbuild(
    startdir: &Path,
    srcdir: &Path,
    arch: &str,
) -> Result<(PkgbuildInfo, InstallHooks)> {
    let pkgbuild_path = startdir.join("PKGBUILD");
    if !pkgbuild_path.is_file() {
        return Err(ApgError::PkgbuildNotFound(pkgbuild_path));
    }

    let output = run_bash_script(
        EXTRACT_SCRIPT,
        &[
            startdir.to_str().unwrap_or("."),
            pkgbuild_path.to_str().unwrap_or("PKGBUILD"),
            srcdir.to_str().unwrap_or("."),
            arch,
        ],
        startdir,
    )?;

    let vars_block = output
        .split(VARS_MARKER)
        .nth(1)
        .and_then(|rest| rest.split(FUNCS_MARKER).next())
        .unwrap_or_default();
    let funcs_block = output
        .split(FUNCS_MARKER)
        .nth(1)
        .and_then(|rest| rest.split(INSTALLFILE_MARKER).next())
        .unwrap_or_default();
    let installfile_block = output.split(INSTALLFILE_MARKER).nth(1).unwrap_or_default();

    let vars = parse_declare_block(vars_block)?;

    let scalar = |name: &str| -> Option<String> {
        match vars.get(name) {
            Some(DeclareValue::Scalar(s)) if !s.is_empty() => Some(s.clone()),
            _ => None,
        }
    };
    let array = |name: &str| -> Vec<String> {
        match vars.get(name) {
            Some(DeclareValue::Array(items)) => items.clone(),
            Some(DeclareValue::Scalar(s)) if !s.is_empty() => vec![s.clone()],
            _ => Vec::new(),
        }
    };

    let pkgname = scalar("pkgname").ok_or_else(|| ApgError::MissingVariable("pkgname".into()))?;
    let pkgver = scalar("pkgver").ok_or_else(|| ApgError::MissingVariable("pkgver".into()))?;
    let pkgrel = scalar("pkgrel").ok_or_else(|| ApgError::MissingVariable("pkgrel".into()))?;
    let pkgdesc = scalar("pkgdesc").unwrap_or_default();

    let funcs: Vec<&str> = funcs_block
        .lines()
        .map(str::trim)
        .filter(|l| !l.is_empty())
        .collect();

    let mut info = PkgbuildInfo {
        pkgname,
        pkgver,
        pkgrel,
        epoch: scalar("epoch"),
        pkgdesc,
        arch: array("arch"),
        url: scalar("url"),
        license: array("license"),
        depends: array("depends"),
        optdepends: array("optdepends"),
        makedepends: array("makedepends"),
        conflicts: array("conflicts"),
        provides: array("provides"),
        replaces: array("replaces"),
        backup: array("backup"),
        source: array("source"),
        install: scalar("install"),
        has_prepare_func: funcs.contains(&"prepare"),
        has_build_func: funcs.contains(&"build"),
        has_package_func: funcs.contains(&"package"),
    };

    if info.arch.is_empty() {
        if let Some(single) = scalar("arch") {
            info.arch = vec![single];
        }
    }

    let install_file_path = installfile_block.trim();
    let hooks = if !install_file_path.is_empty() {
        parse_install_hooks(Path::new(install_file_path), startdir)?
    } else {
        InstallHooks::default()
    };

    Ok((info, hooks))
}

pub fn extract_maintainer_comment(pkgbuild_path: &Path) -> Option<String> {
    let content = fs::read_to_string(pkgbuild_path).ok()?;
    for line in content.lines() {
        let trimmed = line.trim_start_matches('#').trim();
        if let Some(value) = trimmed.strip_prefix("Maintainer:") {
            return Some(value.trim().to_string());
        }
    }
    None
}

pub fn parse_install_hooks(install_file: &Path, cwd: &Path) -> Result<InstallHooks> {
    if !install_file.is_file() {
        return Ok(InstallHooks::default());
    }

    let output = run_bash_script(
        INSTALL_HOOKS_SCRIPT,
        &[install_file.to_str().unwrap_or("")],
        cwd,
    )?;

    let mut hooks = InstallHooks::default();
    for chunk in output.split(HOOK_MARKER_PREFIX).skip(1) {
        let (name_part, body) = chunk
            .split_once(HOOK_MARKER_SUFFIX)
            .ok_or_else(|| ApgError::DeclareParse(chunk.to_string()))?;
        let name = name_part.trim();
        let body = body.trim().to_string();
        match name {
            "pre_install" => hooks.pre_install = Some(body),
            "post_install" => hooks.post_install = Some(body),
            "pre_remove" => hooks.pre_remove = Some(body),
            "post_remove" => hooks.post_remove = Some(body),
            _ => {}
        }
    }

    Ok(hooks)
}
