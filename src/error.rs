use std::path::PathBuf;

use thiserror::Error;

#[derive(Debug, Error)]
pub enum ApgError {
    #[error("PKGBUILD not found at {0}")]
    PkgbuildNotFound(PathBuf),

    #[error("failed to run bash helper: {0}")]
    BashSpawn(#[source] std::io::Error),

    #[error("bash helper exited with status {status}\nstderr:\n{stderr}")]
    BashFailed { status: String, stderr: String },

    #[error("failed to parse declare -p output: {0}")]
    DeclareParse(String),

    #[error("required PKGBUILD variable '{0}' is missing")]
    MissingVariable(String),

    #[error("unsupported architecture '{0}'")]
    UnsupportedArchitecture(String),

    #[error("package() produced no files in pkgdir, nothing to package")]
    EmptyPkgdir,

    #[error("io error at {path}: {source}")]
    Io {
        path: PathBuf,
        #[source]
        source: std::io::Error,
    },

    #[error("json error: {0}")]
    Json(#[from] serde_json::Error),

    #[error("signing error: {0}")]
    Signing(String),

    #[error("invalid key file at {0}")]
    InvalidKey(PathBuf),
}

pub type Result<T> = std::result::Result<T, ApgError>;

pub fn io(path: impl Into<PathBuf>, source: std::io::Error) -> ApgError {
    ApgError::Io {
        path: path.into(),
        source,
    }
}
