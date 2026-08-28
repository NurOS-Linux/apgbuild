#[derive(Debug, Clone, Default)]
#[allow(dead_code)]
pub struct PkgbuildInfo {
    pub pkgname: String,
    pub pkgver: String,
    pub pkgrel: String,
    pub epoch: Option<String>,
    pub pkgdesc: String,
    pub arch: Vec<String>,
    pub url: Option<String>,
    pub license: Vec<String>,
    pub depends: Vec<String>,
    pub optdepends: Vec<String>,
    pub makedepends: Vec<String>,
    pub conflicts: Vec<String>,
    pub provides: Vec<String>,
    pub replaces: Vec<String>,
    pub backup: Vec<String>,
    pub source: Vec<String>,
    pub install: Option<String>,
    pub has_prepare_func: bool,
    pub has_build_func: bool,
    pub has_package_func: bool,
}

impl PkgbuildInfo {
    pub fn full_version(&self) -> String {
        match &self.epoch {
            Some(epoch) if !epoch.is_empty() && epoch != "0" => {
                format!("{}:{}-{}", epoch, self.pkgver, self.pkgrel)
            }
            _ => format!("{}-{}", self.pkgver, self.pkgrel),
        }
    }
}

#[derive(Debug, Clone, Default)]
pub struct InstallHooks {
    pub pre_install: Option<String>,
    pub post_install: Option<String>,
    pub pre_remove: Option<String>,
    pub post_remove: Option<String>,
}
