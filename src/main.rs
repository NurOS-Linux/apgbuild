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

    let recipe_path = pkgbuild::find_recipe(&startdir)?;
    println!("apgbuild: using recipe {}", recipe_path.display());

    let arch = args.arch.unwrap_or_else(default_arch);

    println!("apgbuild: reading recipe");
    let (light_info, _) =
        pkgbuild::parse_pkgbuild(&startdir, &recipe_path, &startdir, &arch, false)?;

    println!("apgbuild: preparing source tree");
    let srcdir = builder::prepare_srcdir(&startdir, &light_info, args.clean)?;

    let (info, hooks) = pkgbuild::parse_pkgbuild(&startdir, &recipe_path, &srcdir, &arch, true)?;

    let package_type = args
        .package_type
        .or_else(|| info.pkgtype.clone())
        .unwrap_or_else(|| "binary".to_string());

    println!(
        "apgbuild: building {} {} for {}",
        info.pkgname,
        info.full_version(),
        arch
    );
    let allow_empty = mapper::allow_empty_pkgdir(&info, &package_type);
    let build_output = builder::run_build_and_package(
        &startdir,
        &recipe_path,
        &srcdir,
        &info,
        &arch,
        allow_empty,
    )?;

    let maintainer = args
        .maintainer
        .or_else(|| info.maintainer.clone())
        .or_else(|| pkgbuild::extract_maintainer_comment(&recipe_path))
        .unwrap_or_else(|| "Unknown".to_string());

    let tags = if args.tags.is_empty() {
        info.tags.clone()
    } else {
        args.tags
    };

    let map_options = MapOptions {
        arch,
        package_type,
        maintainer,
        tags,
    };
    let metadata = mapper::map_to_metadata(&info, &map_options)?;

    let compression = Compression::from_flag(&args.compression).ok_or_else(|| {
        error::ApgError::InvalidArgument(format!(
            "unsupported compression '{}', expected 'xz' or 'zst'",
            args.compression
        ))
    })?;

    let default_name = {
        let arch_label = metadata
            .architecture
            .clone()
            .unwrap_or_else(|| "all".to_string());
        format!("{}-{}-{}.apg", metadata.name, metadata.version, arch_label)
    };

    let output_path = match (args.output, args.output_dir) {
        (Some(output), _) if output.is_dir() => output.join(&default_name),
        (Some(output), _) => output,
        (None, Some(dir)) => dir.join(&default_name),
        (None, None) => PathBuf::from(&default_name),
    };

    if let Some(parent) = output_path.parent() {
        if !parent.as_os_str().is_empty() {
            fs::create_dir_all(parent).map_err(|e| error::io(parent, e))?;
        }
    }

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
    Ok(())
}
