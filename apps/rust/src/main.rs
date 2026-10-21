mod batch;
mod gateway;
mod metrics;
mod prime;
mod server;
#[cfg(test)]
mod test_support;

use std::env;
use std::io;
use std::str::FromStr;

pub struct Config {
    mode: String,
    port: u16,
    bits: u64,
    count: u64,
    concurrency: usize,
    rounds: u32,
    run_id: String,
    results_dir: String,
    downstream_url: String,
}

impl Config {
    fn from_env() -> Self {
        Self {
            mode: env_or("MODE", "server".to_string()),
            port: env_or("PORT", 8080),
            bits: env_or("BITS", 1024),
            count: env_or("COUNT", 200),
            concurrency: env_or("CONCURRENCY", metrics::cpus()),
            rounds: env_or("ROUNDS", 20),
            run_id: env_or("RUN_ID", "local".to_string()),
            results_dir: env_or("RESULTS_DIR", "results".to_string()),
            downstream_url: env_or("DOWNSTREAM_URL", "http://mock:8080".to_string()),
        }
    }
}

fn env_or<T: FromStr + PartialEq + Default>(key: &str, fallback: T) -> T {
    env::var(key)
        .ok()
        .and_then(|value| value.parse().ok())
        .filter(|value| *value != T::default())
        .unwrap_or(fallback)
}

fn main() -> io::Result<()> {
    let config = Config::from_env();
    match config.mode.as_str() {
        "server" => tokio::runtime::Runtime::new()?.block_on(server::run(&config)),
        "batch" => batch::run(&config),
        other => Err(io::Error::other(format!(
            "unknown MODE {other}, expected server or batch"
        ))),
    }
}
