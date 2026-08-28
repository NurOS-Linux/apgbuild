use crate::error::{ApgError, Result};
use crate::metadata::Metadata;
use crate::pkgbuild::PkgbuildInfo;

pub struct MapOptions {
    pub arch: String,
    pub package_type: String,
    pub maintainer: String,
    pub tags: Vec<String>,
}

pub fn map_to_metadata(info: &PkgbuildInfo, opts: &MapOptions) -> Result<Metadata> {
    let architecture = map_architecture(&info.arch, &opts.arch)?;

    let license = if info.license.is_empty() {
        None
    } else {
        Some(info.license.join(" AND "))
    };

    Ok(Metadata {
        name: info.pkgname.clone(),
        version: info.full_version(),
        package_type: opts.package_type.clone(),
        architecture,
        description: info.pkgdesc.clone(),
        maintainer: opts.maintainer.clone(),
        license,
        tags: opts.tags.clone(),
        homepage: info.url.clone().unwrap_or_default(),
        dependencies: info
            .depends
            .iter()
            .map(|d| normalize_dependency(d))
            .collect(),
        conflicts: info.conflicts.clone(),
        provides: info.provides.clone(),
        replaces: info.replaces.clone(),
        conf: info.backup.iter().map(|b| normalize_conf_path(b)).collect(),
    })
}

fn map_architecture(pkgbuild_arch: &[String], target_arch: &str) -> Result<Option<String>> {
    if pkgbuild_arch.iter().any(|a| a == "any") {
        return Ok(Some("all".to_string()));
    }

    let mapped_target = match target_arch {
        "x86_64" => "x86_64",
        "aarch64" => "aarch64",
        "riscv64" => "riscv64",
        other => return Err(ApgError::UnsupportedArchitecture(other.to_string())),
    };

    if pkgbuild_arch.is_empty() || pkgbuild_arch.iter().any(|a| a == mapped_target) {
        Ok(Some(mapped_target.to_string()))
    } else {
        Err(ApgError::UnsupportedArchitecture(format!(
            "PKGBUILD declares arch={:?}, but target is {}",
            pkgbuild_arch, target_arch
        )))
    }
}

fn normalize_dependency(dep: &str) -> String {
    for op in ["==", ">=", "<=", ">", "<", "="] {
        if let Some(pos) = dep.find(op) {
            let (name, rest) = dep.split_at(pos);
            let value = &rest[op.len()..];
            return format!("{} {} {}", name.trim(), op, value.trim());
        }
    }
    dep.trim().to_string()
}

fn normalize_conf_path(path: &str) -> String {
    if path.starts_with('/') || path.starts_with("$HOME") || path.starts_with('~') {
        path.to_string()
    } else {
        format!("/{}", path)
    }
}
