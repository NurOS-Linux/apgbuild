use std::fs::File;
use std::io::Write;
use std::path::{Path, PathBuf};

use tar::{Builder, Header};

use crate::error::{self, Result};
use crate::metadata::Metadata;
use crate::pkgbuild::InstallHooks;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Compression {
    Xz,
    Zstd,
}

impl Compression {
    pub fn from_flag(flag: &str) -> Option<Self> {
        match flag {
            "xz" => Some(Compression::Xz),
            "zst" | "zstd" => Some(Compression::Zstd),
            _ => None,
        }
    }
}

pub struct PackageInputs<'a> {
    pub metadata: &'a Metadata,
    pub data_dir: &'a Path,
    pub hooks: &'a InstallHooks,
    pub compression: Compression,
    pub output_path: &'a Path,
}

pub fn build_package(inputs: &PackageInputs) -> Result<()> {
    let tar_bytes = write_tar(inputs)?;

    let compressed = match inputs.compression {
        Compression::Xz => {
            let mut encoder = xz2::write::XzEncoder::new(Vec::new(), 9);
            encoder
                .write_all(&tar_bytes)
                .map_err(|e| error::io(inputs.output_path, e))?;
            encoder
                .finish()
                .map_err(|e| error::io(inputs.output_path, e))?
        }
        Compression::Zstd => zstd::stream::encode_all(tar_bytes.as_slice(), 19)
            .map_err(|e| error::io(inputs.output_path, e))?,
    };

    let mut file =
        File::create(inputs.output_path).map_err(|e| error::io(inputs.output_path, e))?;
    file.write_all(&compressed)
        .map_err(|e| error::io(inputs.output_path, e))?;

    Ok(())
}

fn write_tar(inputs: &PackageInputs) -> Result<Vec<u8>> {
    let mut builder = Builder::new(Vec::new());
    builder.mode(tar::HeaderMode::Deterministic);

    let metadata_json = serde_json::to_vec_pretty(inputs.metadata)?;
    append_file_bytes(&mut builder, "metadata.json", &metadata_json, 0o644)?;

    builder
        .append_dir_all("data", inputs.data_dir)
        .map_err(|e| error::io(inputs.data_dir, e))?;

    for (name, body) in [
        ("pre-install", &inputs.hooks.pre_install),
        ("post-install", &inputs.hooks.post_install),
        ("pre-remove", &inputs.hooks.pre_remove),
        ("post-remove", &inputs.hooks.post_remove),
    ] {
        if let Some(func_source) = body {
            let script = render_hook_script(name, func_source);
            let archive_path = format!("scripts/{}", name);
            append_file_bytes(&mut builder, &archive_path, script.as_bytes(), 0o755)?;
        }
    }

    builder
        .into_inner()
        .map_err(|e| error::io(PathBuf::from("archive"), e))
}

fn append_file_bytes<W: Write>(
    builder: &mut Builder<W>,
    archive_path: &str,
    data: &[u8],
    mode: u32,
) -> Result<()> {
    let mut header = Header::new_gnu();
    header.set_size(data.len() as u64);
    header.set_mode(mode);
    header.set_cksum();
    builder
        .append_data(&mut header, archive_path, data)
        .map_err(|e| error::io(PathBuf::from(archive_path), e))
}

fn hook_function_name(archive_name: &str) -> &str {
    match archive_name {
        "pre-install" => "pre_install",
        "post-install" => "post_install",
        "pre-remove" => "pre_remove",
        "post-remove" => "post_remove",
        other => other,
    }
}

fn render_hook_script(archive_name: &str, func_source: &str) -> String {
    let func_name = hook_function_name(archive_name);
    format!(
        "#!/bin/bash\nset -e\n\n{}\n\n{} \"$@\"\n",
        func_source.trim(),
        func_name
    )
}
