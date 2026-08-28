use serde::Serialize;

#[derive(Debug, Clone, Serialize)]
pub struct Metadata {
    pub name: String,
    pub version: String,
    #[serde(rename = "type")]
    pub package_type: String,
    pub architecture: Option<String>,
    pub description: String,
    pub maintainer: String,
    pub license: Option<String>,
    pub tags: Vec<String>,
    pub homepage: String,
    pub dependencies: Vec<String>,
    pub conflicts: Vec<String>,
    pub provides: Vec<String>,
    pub replaces: Vec<String>,
    pub conf: Vec<String>,
}
