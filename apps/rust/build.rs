use std::env;
use std::process::Command;

fn main() {
    let rustc = env::var("RUSTC").unwrap_or_else(|_| "rustc".into());
    let output = Command::new(rustc)
        .arg("--version")
        .output()
        .expect("rustc --version");
    let version = String::from_utf8_lossy(&output.stdout);
    let number = version.split_whitespace().nth(1).unwrap_or("unknown");
    println!("cargo:rustc-env=RUSTC_VERSION={number}");
}
