mod builder;
mod cli;
mod error;
mod mapper;
mod metadata;
mod packager;
mod pkgbuild;
mod signer;

use std::fs;
use std::path::PathBuf;

use clap::Parser;
use dryoc::sign::SigningKeyPair;

use cli::{BuildArgs, Cli, Command, KeygenArgs, VerifyArgs};
use error::ApgError;
use mapper::MapOptions;
use packager::{Compression, PackageInputs};

fn main() {
    let cli = Cli::parse();

    let result = match cli.command {
        Command::Build(args) => run_build(args),
        Command::Keygen(args) => run_keygen(args),
        Command::Verify(args) => run_verify(args),
    };

    if let Err(err) = result {
        eprintln!("apgbuild: error: {}", err);
        std::process::exit(1);
    }
}

fn default_arch() -> String {
    match std::env::consts::ARCH {
        "riscv64gc" => "riscv64".to_string(),
        other => other.to_string(),
    }
}

fn run_build(args: BuildArgs) -> error::Result<()> {
    let startdir = args
        .dir
        .canonicalize()
        .map_err(|e| error::io(&args.dir, e))?;

    let pkgbuild_path = startdir.join("PKGBUILD");
    if !pkgbuild_path.is_file() {
        return Err(ApgError::PkgbuildNotFound(pkgbuild_path));
    }

    let arch = args.arch.unwrap_or_else(default_arch);

    println!("apgbuild: preparing source tree");
    let srcdir = builder::prepare_srcdir(&startdir)?;

    println!("apgbuild: parsing PKGBUILD");
    let (info, hooks) = pkgbuild::parse_pkgbuild(&startdir, &srcdir, &arch)?;

    println!(
        "apgbuild: building {} {} for {}",
        info.pkgname,
        info.full_version(),
        arch
    );
    let build_output = builder::run_build_and_package(&startdir, &srcdir, &info, &arch)?;

    let maintainer = args.maintainer.unwrap_or_else(|| {
        pkgbuild::extract_maintainer_comment(&pkgbuild_path)
            .unwrap_or_else(|| "Unknown".to_string())
    });

    let map_options = MapOptions {
        arch,
        package_type: args.package_type,
        maintainer,
        tags: args.tags,
    };
    let metadata = mapper::map_to_metadata(&info, &map_options)?;

    let compression = Compression::from_flag(&args.compression).ok_or_else(|| {
        ApgError::Signing(format!(
            "unsupported compression '{}', expected 'xz' or 'zst'",
            args.compression
        ))
    })?;

    let output_path = args.output.unwrap_or_else(|| {
        let arch_label = metadata
            .architecture
            .clone()
            .unwrap_or_else(|| "all".to_string());
        PathBuf::from(format!(
            "{}-{}-{}.apg",
            metadata.name, metadata.version, arch_label
        ))
    });

    println!("apgbuild: packaging {}", output_path.display());
    let package_inputs = PackageInputs {
        metadata: &metadata,
        data_dir: &build_output.pkgdir,
        hooks: &hooks,
        compression,
        output_path: &output_path,
    };
    packager::build_package(&package_inputs)?;

    if let Some(sign_key_path) = args.sign_key {
        println!("apgbuild: signing package");
        let secret_key = signer::load_secret_key(&sign_key_path)?;
        let keypair = SigningKeyPair::from_secret_key(secret_key);
        let signature = signer::sign_file(&output_path, &keypair.secret_key)?;

        let sig_path = PathBuf::from(format!("{}.sig", output_path.display()));
        signer::write_signature(&signature, &sig_path)?;

        let pub_path = PathBuf::from(format!("{}.pub.key", output_path.display()));
        signer::write_public_key(&keypair.public_key, &pub_path)?;

        println!("apgbuild: wrote {}", sig_path.display());
        println!("apgbuild: wrote {}", pub_path.display());
    }

    println!("apgbuild: done, wrote {}", output_path.display());
    Ok(())
}

fn run_keygen(args: KeygenArgs) -> error::Result<()> {
    let keypair = signer::generate_keypair();

    let prefix = args.output_prefix.to_string_lossy().to_string();
    let public_path = PathBuf::from(format!("{}.pub.key", prefix));
    let secret_path = PathBuf::from(format!("{}.secret", prefix));

    signer::write_keypair(&keypair, &public_path, &secret_path)?;

    println!("apgbuild: wrote {}", public_path.display());
    println!(
        "apgbuild: wrote {} (keep this secret)",
        secret_path.display()
    );
    Ok(())
}

fn run_verify(args: VerifyArgs) -> error::Result<()> {
    let signature_path = args
        .signature
        .unwrap_or_else(|| PathBuf::from(format!("{}.sig", args.package.display())));

    let public_key = signer::load_public_key(&args.pubkey)?;
    let signature = signer::read_signature(&signature_path)?;

    signer::verify_file(&args.package, &signature, &public_key)?;

    println!("apgbuild: signature OK for {}", args.package.display());
    let _ = fs::metadata(&args.package);
    Ok(())
}
