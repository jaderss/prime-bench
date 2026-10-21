use std::fs::{self, File};
use std::io::{self, BufWriter, Write};
use std::path::Path;
use std::sync::mpsc::{self, RecvTimeoutError, Sender};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

const HEADER: &str = "kind,lang,lang_version,mode,run_id,elapsed_s,cpus,bits,concurrency,completed,errors,\
throughput_ops_s,latency_p50_ms,latency_p95_ms,latency_p99_ms,latency_max_ms,rss_mb,cpu_pct";
const CLOCK_TICKS_PER_SECOND: f64 = 100.0;
const MIN_INTERVAL: Duration = Duration::from_millis(250);

pub const LANG: &str = "rust";
pub const LANG_VERSION: &str = env!("RUSTC_VERSION");

pub fn cpus() -> usize {
    thread::available_parallelism().map_or(1, |n| n.get())
}

pub struct ProcessStats {
    pub rss_mb: f64,
    pub cpu_seconds: f64,
}

impl ProcessStats {
    pub fn read() -> Self {
        Self {
            rss_mb: read_rss_mb().unwrap_or(0.0),
            cpu_seconds: read_cpu_seconds().unwrap_or(0.0),
        }
    }
}

fn read_rss_mb() -> Option<f64> {
    let status = fs::read_to_string("/proc/self/status").ok()?;
    let line = status.lines().find(|line| line.starts_with("VmRSS:"))?;
    let kb: f64 = line.split_whitespace().nth(1)?.parse().ok()?;
    Some(kb / 1024.0)
}

fn read_cpu_seconds() -> Option<f64> {
    let stat = fs::read_to_string("/proc/self/stat").ok()?;
    let fields: Vec<&str> = stat[stat.rfind(')')? + 1..].split_whitespace().collect();
    let utime: f64 = fields.get(11)?.parse().ok()?;
    let stime: f64 = fields.get(12)?.parse().ok()?;
    Some((utime + stime) / CLOCK_TICKS_PER_SECOND)
}

#[derive(Default)]
struct Latencies {
    interval: Vec<f64>,
    all: Vec<f64>,
}

#[derive(Default)]
pub struct Recorder {
    latencies: Mutex<Latencies>,
}

impl Recorder {
    pub fn record(&self, latency: Duration) {
        let ms = latency.as_secs_f64() * 1000.0;
        let mut latencies = self.latencies.lock().unwrap();
        latencies.interval.push(ms);
        latencies.all.push(ms);
    }

    fn drain(&self) -> Vec<f64> {
        std::mem::take(&mut self.latencies.lock().unwrap().interval)
    }

    fn total(&self) -> Vec<f64> {
        self.latencies.lock().unwrap().all.clone()
    }
}

pub struct RunInfo {
    pub mode: &'static str,
    pub run_id: String,
    pub bits: u64,
    pub concurrency: usize,
}

pub struct Row {
    kind: &'static str,
    elapsed_s: f64,
    sorted: Vec<f64>,
    wall_s: f64,
    rss_mb: f64,
    cpu_pct: f64,
}

impl Row {
    fn new(
        kind: &'static str,
        elapsed_s: f64,
        mut latencies: Vec<f64>,
        wall_s: f64,
        rss_mb: f64,
        cpu_pct: f64,
    ) -> Self {
        latencies.sort_by(f64::total_cmp);
        Self {
            kind,
            elapsed_s,
            sorted: latencies,
            wall_s,
            rss_mb,
            cpu_pct,
        }
    }

    fn throughput(&self) -> f64 {
        self.sorted.len() as f64 / self.wall_s
    }
}

impl std::fmt::Display for Row {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(
            f,
            "completed={} errors=0 throughput={:.2}/s p50={:.2}ms p99={:.2}ms peak_rss={:.1}MB cpu={:.0}%",
            self.sorted.len(),
            self.throughput(),
            percentile(&self.sorted, 50.0),
            percentile(&self.sorted, 99.0),
            self.rss_mb,
            self.cpu_pct
        )
    }
}

pub fn percentile(sorted: &[f64], p: f64) -> f64 {
    if sorted.is_empty() {
        return 0.0;
    }
    let rank = (p / 100.0 * sorted.len() as f64).ceil() as usize;
    sorted[rank.max(1) - 1]
}

struct State {
    info: RunInfo,
    recorder: Arc<Recorder>,
    writer: BufWriter<File>,
    start: Instant,
    start_cpu: f64,
    last: Instant,
    last_cpu: f64,
    peak_rss: f64,
}

impl State {
    fn sample(&mut self) -> io::Result<()> {
        let now = Instant::now();
        let stats = ProcessStats::read();
        let wall = (now - self.last).as_secs_f64();
        self.peak_rss = self.peak_rss.max(stats.rss_mb);
        let row = Row::new(
            "interval",
            (now - self.start).as_secs_f64(),
            self.recorder.drain(),
            wall,
            stats.rss_mb,
            (stats.cpu_seconds - self.last_cpu) / wall * 100.0,
        );
        self.last = now;
        self.last_cpu = stats.cpu_seconds;
        self.write(&row)
    }

    fn write(&mut self, row: &Row) -> io::Result<()> {
        let s = &row.sorted;
        writeln!(
            self.writer,
            "{},{LANG},{LANG_VERSION},{},{},{:.3},{},{},{},{},0,{:.3},{:.3},{:.3},{:.3},{:.3},{:.3},{:.3}",
            row.kind,
            self.info.mode,
            self.info.run_id,
            row.elapsed_s,
            cpus(),
            self.info.bits,
            self.info.concurrency,
            s.len(),
            row.throughput(),
            percentile(s, 50.0),
            percentile(s, 95.0),
            percentile(s, 99.0),
            percentile(s, 100.0),
            row.rss_mb,
            row.cpu_pct
        )?;
        self.writer.flush()
    }
}

pub struct Sampler {
    state: Arc<Mutex<State>>,
    stop: Sender<()>,
    ticker: JoinHandle<()>,
}

impl Sampler {
    pub fn start(dir: &Path, info: RunInfo, recorder: Arc<Recorder>) -> io::Result<Self> {
        fs::create_dir_all(dir)?;
        let file = File::create(dir.join(format!("{}_{}_{LANG}.csv", info.run_id, info.mode)))?;
        let mut writer = BufWriter::new(file);
        writeln!(writer, "{HEADER}")?;
        let now = Instant::now();
        let cpu = ProcessStats::read().cpu_seconds;
        let state = Arc::new(Mutex::new(State {
            info,
            recorder,
            writer,
            start: now,
            start_cpu: cpu,
            last: now,
            last_cpu: cpu,
            peak_rss: 0.0,
        }));
        let (stop, ticks) = mpsc::channel();
        let ticking = Arc::clone(&state);
        let ticker = thread::spawn(move || {
            while let Err(RecvTimeoutError::Timeout) = ticks.recv_timeout(Duration::from_secs(1)) {
                let _ = ticking.lock().unwrap().sample();
            }
        });
        Ok(Self {
            state,
            stop,
            ticker,
        })
    }

    pub fn finish(self) -> io::Result<Row> {
        let _ = self.stop.send(());
        let _ = self.ticker.join();
        let mut state = self.state.lock().unwrap();
        if state.last.elapsed() >= MIN_INTERVAL {
            state.sample()?;
        }
        let stats = ProcessStats::read();
        let wall = state.start.elapsed().as_secs_f64();
        let cpu_pct = (stats.cpu_seconds - state.start_cpu) / wall * 100.0;
        let peak = state.peak_rss.max(stats.rss_mb);
        let summary = Row::new("summary", wall, state.recorder.total(), wall, peak, cpu_pct);
        state.write(&summary)?;
        Ok(summary)
    }
}

#[cfg(test)]
mod tests {
    use super::percentile;

    #[test]
    fn percentile_uses_nearest_rank() {
        let sorted: Vec<f64> = (1..=10).map(f64::from).collect();
        for (p, expected) in [
            (50.0, 5.0),
            (95.0, 10.0),
            (99.0, 10.0),
            (100.0, 10.0),
            (10.0, 1.0),
        ] {
            assert_eq!(percentile(&sorted, p), expected);
        }
        assert_eq!(percentile(&[], 50.0), 0.0);
    }
}
