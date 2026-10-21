use std::io;
use std::path::Path;
use std::sync::Arc;
use std::sync::atomic::{AtomicI64, Ordering};
use std::thread;
use std::time::Instant;

use crate::Config;
use crate::metrics::{LANG_VERSION, Recorder, RunInfo, Sampler};
use crate::prime::Generator;

pub fn run(config: &Config) -> io::Result<()> {
    let recorder = Arc::new(Recorder::default());
    let info = RunInfo {
        mode: "batch",
        run_id: config.run_id.clone(),
        bits: config.bits,
        concurrency: config.concurrency,
    };
    let sampler = Sampler::start(Path::new(&config.results_dir), info, Arc::clone(&recorder))?;
    let remaining = AtomicI64::new(config.count as i64);
    let generator = Generator::new(config.rounds);
    thread::scope(|scope| {
        for _ in 0..config.concurrency {
            scope.spawn(|| {
                while remaining.fetch_sub(1, Ordering::Relaxed) > 0 {
                    let start = Instant::now();
                    generator.probable_prime(config.bits);
                    recorder.record(start.elapsed());
                }
            });
        }
    });
    let summary = sampler.finish()?;
    println!(
        "rust {LANG_VERSION} batch bits={} count={} workers={} {summary}",
        config.bits, config.count, config.concurrency
    );
    Ok(())
}
