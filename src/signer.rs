use std::fs;
use std::io::Read;
use std::path::Path;

use dryoc::sign::{IncrementalSigner, PublicKey, SecretKey, Signature, SigningKeyPair};
use dryoc::types::Bytes;

use crate::error::{self, ApgError, Result};

pub fn generate_keypair() -> SigningKeyPair<PublicKey, SecretKey> {
    SigningKeyPair::gen()
}

pub fn write_keypair(
    keypair: &SigningKeyPair<PublicKey, SecretKey>,
    public_path: &Path,
    secret_path: &Path,
) -> Result<()> {
    let public_hex = hex::encode(keypair.public_key.as_slice());
    let secret_hex = hex::encode(keypair.secret_key.as_slice());

    fs::write(public_path, format!("{}\n", public_hex)).map_err(|e| error::io(public_path, e))?;
    fs::write(secret_path, format!("{}\n", secret_hex)).map_err(|e| error::io(secret_path, e))?;

    set_owner_only_permissions(secret_path)?;
    Ok(())
}

#[cfg(unix)]
fn set_owner_only_permissions(path: &Path) -> Result<()> {
    use std::os::unix::fs::PermissionsExt;
    fs::set_permissions(path, fs::Permissions::from_mode(0o600)).map_err(|e| error::io(path, e))
}

#[cfg(not(unix))]
fn set_owner_only_permissions(_path: &Path) -> Result<()> {
    Ok(())
}

pub fn write_public_key(public_key: &PublicKey, path: &Path) -> Result<()> {
    let encoded = hex::encode(public_key.as_slice());
    fs::write(path, format!("{}\n", encoded)).map_err(|e| error::io(path, e))
}

pub fn load_secret_key(path: &Path) -> Result<SecretKey> {
    let hex_str = fs::read_to_string(path).map_err(|e| error::io(path, e))?;
    let bytes =
        hex::decode(hex_str.trim()).map_err(|_| ApgError::InvalidKey(path.to_path_buf()))?;
    SecretKey::try_from(bytes.as_slice()).map_err(|_| ApgError::InvalidKey(path.to_path_buf()))
}

pub fn load_public_key(path: &Path) -> Result<PublicKey> {
    let hex_str = fs::read_to_string(path).map_err(|e| error::io(path, e))?;
    let bytes =
        hex::decode(hex_str.trim()).map_err(|_| ApgError::InvalidKey(path.to_path_buf()))?;
    PublicKey::try_from(bytes.as_slice()).map_err(|_| ApgError::InvalidKey(path.to_path_buf()))
}

pub fn sign_file(path: &Path, secret_key: &SecretKey) -> Result<Signature> {
    let mut signer = IncrementalSigner::new();
    feed_file(path, &mut signer)?;
    signer
        .finalize::<Signature, SecretKey>(secret_key)
        .map_err(|e| ApgError::Signing(e.to_string()))
}

pub fn verify_file(path: &Path, signature: &Signature, public_key: &PublicKey) -> Result<()> {
    let mut signer = IncrementalSigner::new();
    feed_file(path, &mut signer)?;
    signer
        .verify(signature, public_key)
        .map_err(|e| ApgError::Signing(e.to_string()))
}

fn feed_file(path: &Path, signer: &mut IncrementalSigner) -> Result<()> {
    let mut file = fs::File::open(path).map_err(|e| error::io(path, e))?;
    let mut buffer = [0u8; 65536];
    loop {
        let read = file.read(&mut buffer).map_err(|e| error::io(path, e))?;
        if read == 0 {
            break;
        }
        signer.update(&buffer[..read].to_vec());
    }
    Ok(())
}

pub fn write_signature(signature: &Signature, path: &Path) -> Result<()> {
    let encoded = hex::encode(signature.as_slice());
    fs::write(path, format!("{}\n", encoded)).map_err(|e| error::io(path, e))
}

pub fn read_signature(path: &Path) -> Result<Signature> {
    let hex_str = fs::read_to_string(path).map_err(|e| error::io(path, e))?;
    let bytes =
        hex::decode(hex_str.trim()).map_err(|_| ApgError::InvalidKey(path.to_path_buf()))?;
    Signature::try_from(bytes.as_slice()).map_err(|_| ApgError::InvalidKey(path.to_path_buf()))
}
