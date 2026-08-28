pub mod bash;
pub mod model;
pub mod parser;

pub use bash::run_bash_script as bash_run;
pub use model::{InstallHooks, PkgbuildInfo};
pub use parser::{extract_maintainer_comment, parse_pkgbuild};
