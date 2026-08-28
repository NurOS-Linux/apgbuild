use std::collections::HashMap;
use std::path::Path;
use std::process::Command;

use crate::error::{ApgError, Result};

#[derive(Debug, Clone)]
pub enum DeclareValue {
    Scalar(String),
    Array(Vec<String>),
}

pub fn run_bash_script(script: &str, args: &[&str], cwd: &Path) -> Result<String> {
    let output = Command::new("bash")
        .arg("--noprofile")
        .arg("--norc")
        .arg("-c")
        .arg(script)
        .arg("apgbuild")
        .args(args)
        .current_dir(cwd)
        .output()
        .map_err(ApgError::BashSpawn)?;

    if !output.status.success() {
        return Err(ApgError::BashFailed {
            status: output.status.to_string(),
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        });
    }

    Ok(String::from_utf8_lossy(&output.stdout).into_owned())
}

pub fn parse_declare_block(block: &str) -> Result<HashMap<String, DeclareValue>> {
    let mut map = HashMap::new();
    for line in block.lines() {
        if line.trim().is_empty() {
            continue;
        }
        if let Some((name, value)) = parse_declare_line(line)? {
            map.insert(name, value);
        }
    }
    Ok(map)
}

fn parse_declare_line(line: &str) -> Result<Option<(String, DeclareValue)>> {
    let rest = match line.strip_prefix("declare ") {
        Some(rest) => rest,
        None => return Ok(None),
    };

    let space_idx = rest
        .find(' ')
        .ok_or_else(|| ApgError::DeclareParse(line.to_string()))?;
    let flags = &rest[..space_idx];
    let remainder = rest[space_idx + 1..].trim_start();
    let is_array = flags.contains('a') || flags.contains('A');

    let eq_idx = match remainder.find('=') {
        Some(idx) => idx,
        None => {
            return Ok(Some((
                remainder.trim().to_string(),
                DeclareValue::Scalar(String::new()),
            )))
        }
    };
    let name = remainder[..eq_idx].to_string();
    let value_str = &remainder[eq_idx + 1..];

    if is_array {
        Ok(Some((
            name,
            DeclareValue::Array(parse_array_value(value_str)?),
        )))
    } else {
        Ok(Some((
            name,
            DeclareValue::Scalar(parse_scalar_value(value_str)?),
        )))
    }
}

fn parse_scalar_value(value: &str) -> Result<String> {
    let value = value.trim();
    if let Some(stripped) = value.strip_prefix('"') {
        let (unescaped, _) = unescape_quoted(stripped)?;
        Ok(unescaped)
    } else {
        Ok(value.to_string())
    }
}

fn parse_array_value(value: &str) -> Result<Vec<String>> {
    let value = value.trim();
    let inner = value
        .strip_prefix('(')
        .and_then(|v| v.strip_suffix(')'))
        .ok_or_else(|| ApgError::DeclareParse(value.to_string()))?;

    let mut elements = Vec::new();
    let chars: Vec<char> = inner.chars().collect();
    let mut i = 0;
    while i < chars.len() {
        while i < chars.len() && chars[i].is_whitespace() {
            i += 1;
        }
        if i >= chars.len() {
            break;
        }
        if chars[i] != '[' {
            return Err(ApgError::DeclareParse(inner.to_string()));
        }
        while i < chars.len() && chars[i] != ']' {
            i += 1;
        }
        i += 1;
        if i >= chars.len() || chars[i] != '=' {
            return Err(ApgError::DeclareParse(inner.to_string()));
        }
        i += 1;
        if i >= chars.len() || chars[i] != '"' {
            return Err(ApgError::DeclareParse(inner.to_string()));
        }
        i += 1;
        let remainder: String = chars[i..].iter().collect();
        let (value, consumed) = unescape_quoted(&remainder)?;
        elements.push(value);
        i += consumed;
    }

    Ok(elements)
}

fn unescape_quoted(text: &str) -> Result<(String, usize)> {
    let chars: Vec<char> = text.chars().collect();
    let mut out = String::new();
    let mut i = 0;
    while i < chars.len() {
        match chars[i] {
            '"' => return Ok((out, i + 1)),
            '\\' if i + 1 < chars.len() => {
                out.push(chars[i + 1]);
                i += 2;
            }
            c => {
                out.push(c);
                i += 1;
            }
        }
    }
    Err(ApgError::DeclareParse(text.to_string()))
}
