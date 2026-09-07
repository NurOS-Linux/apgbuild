use std::fs;
use std::path::{Path, PathBuf};

use super::bash::{parse_declare_block, run_bash_script, DeclareValue};
use super::model::{InstallHooks, PkgbuildInfo};
use crate::error::{ApgError, Result};

const VARS_MARKER: &str = "###APGBUILD:VARS###";
const FUNCS_MARKER: &str = "###APGBUILD:FUNCS###";
const INSTALLFILE_MARKER: &str = "###APGBUILD:INSTALLFILE###";
const NATIVEHOOKS_MARKER: &str = "###APGBUILD:NATIVEHOOKS###";
const HOOK_MARKER_PREFIX: &str = "###APGBUILD:HOOK:";
const HOOK_MARKER_SUFFIX: &str = "###";

const RECIPE_NAMES: &[&str] = &["APGBUILD", "PKGBUILD"];

const EXTRACT_SCRIPT: &str = r####"
set -euo pipefail
STARTDIR="$1"
RECIPE_PATH="$2"
SRCDIR="$3"
CARCH="${4:-x86_64}"
RUN_PKGVER="${5:-1}"
cd "$STARTDIR"
source "$RECIPE_PATH"
if [ "$RUN_PKGVER" = "1" ] && declare -f pkgver >/dev/null 2>&1; then
  cd "$SRCDIR"
  pkgver="$(pkgver)"
  cd "$STARTDIR"
fi
echo '###APGBUILD:VARS###'
declare -p pkgname pkgver pkgrel epoch pkgdesc pkgtype maintainer arch url license tags depends optdepends makedepends conflicts provides replaces backup conf source sha256sums install 2>/dev/null || true
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
echo '###APGBUILD:NATIVEHOOKS###'
for hookname in pre_install post_install pre_remove post_remove; do
  if declare -f "$hookname" >/dev/null 2>&1; then
    echo "###APGBUILD:HOOK:${hookname}###"
    declare -f "$hookname"
  fi
done
"####;

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

pub fn find_recipe(startdir: &Path) -> Result<PathBuf> {
    for name in RECIPE_NAMES {
        let candidate = startdir.join(name);
        if candidate.is_file() {
            return Ok(candidate);
        }
    }
    Err(ApgError::PkgbuildNotFound(startdir.join("APGBUILD")))
}

pub fn parse_pkgbuild(
    startdir: &Path,
    recipe_path: &Path,
    srcdir: &Path,
    arch: &str,
    run_pkgver: bool,
) -> Result<(PkgbuildInfo, InstallHooks)> {
    let output = run_bash_script(
        EXTRACT_SCRIPT,
        &[
            startdir.to_str().unwrap_or("."),
            recipe_path.to_str().unwrap_or("PKGBUILD"),
            srcdir.to_str().unwrap_or("."),
            arch,
            if run_pkgver { "1" } else { "0" },
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
    let installfile_block = output
        .split(INSTALLFILE_MARKER)
        .nth(1)
        .and_then(|rest| rest.split(NATIVEHOOKS_MARKER).next())
        .unwrap_or_default();
    let nativehooks_block = output.split(NATIVEHOOKS_MARKER).nth(1).unwrap_or_default();

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
        pkgtype: scalar("pkgtype"),
        maintainer: scalar("maintainer"),
        arch: array("arch"),
        url: scalar("url"),
        license: array("license"),
        tags: array("tags"),
        depends: array("depends"),
        optdepends: array("optdepends"),
        makedepends: array("makedepends"),
        conflicts: array("conflicts"),
        provides: array("provides"),
        replaces: array("replaces"),
        backup: array("backup"),
        conf: array("conf"),
        source: array("source"),
        sha256sums: array("sha256sums"),
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

    let native_hooks = parse_hook_chunks(nativehooks_block)?;

    let install_file_path = installfile_block.trim();
    let fallback_hooks = if !install_file_path.is_empty() {
        parse_install_hooks(Path::new(install_file_path), startdir)?
    } else {
        InstallHooks::default()
    };

    let hooks = native_hooks.with_fallback(fallback_hooks);

    Ok((info, hooks))
}

pub fn extract_maintainer_comment(recipe_path: &Path) -> Option<String> {
    let content = fs::read_to_string(recipe_path).ok()?;
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

    parse_hook_chunks(&output)
}

fn parse_hook_chunks(text: &str) -> Result<InstallHooks> {
    let mut hooks = InstallHooks::default();
    for chunk in text.split(HOOK_MARKER_PREFIX).skip(1) {
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
