use std::path::PathBuf;

use clap::{Parser, Subcommand};

#[derive(Debug, Parser)]
#[command(name = "apgbuild", version, about = "Converts Arch Linux PKGBUILD packages into APGv2 packages for NurOS", long_about = None)]
pub struct Cli {
    #[command(subcommand)]
    pub command: Command,
}

#[derive(Debug, Subcommand)]
pub enum Command {
    Build(BuildArgs),
    Keygen(KeygenArgs),
    Verify(VerifyArgs),
}

#[derive(Debug, Parser)]
pub struct BuildArgs {
    pub dir: PathBuf,

    #[arg(short = 'o', long = "output")]
    pub output: Option<PathBuf>,

    #[arg(long = "output-dir")]
    pub output_dir: Option<PathBuf>,

    #[arg(long = "compression", default_value = "zst")]
    pub compression: String,

    #[arg(long = "arch")]
    pub arch: Option<String>,

    #[arg(long = "type")]
    pub package_type: Option<String>,

    #[arg(long = "maintainer")]
    pub maintainer: Option<String>,

    #[arg(long = "tag", value_delimiter = ',')]
    pub tags: Vec<String>,

    #[arg(long = "sign-key")]
    pub sign_key: Option<PathBuf>,

    #[arg(short = 'c', long = "clean")]
    pub clean: bool,
}

#[derive(Debug, Parser)]
pub struct KeygenArgs {
    #[arg(short = 'o', long = "output")]
    pub output_prefix: PathBuf,
}

#[derive(Debug, Parser)]
pub struct VerifyArgs {
    pub package: PathBuf,

    #[arg(long = "pubkey")]
    pub pubkey: PathBuf,

    #[arg(long = "signature")]
    pub signature: Option<PathBuf>,
}
