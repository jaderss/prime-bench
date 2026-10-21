use std::io;
use std::time::Instant;

use axum::extract::{Query, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use serde::Deserialize;
use serde_json::json;
use tokio::sync::oneshot;

use crate::Config;
use crate::metrics::{LANG, LANG_VERSION, ProcessStats, cpus};
use crate::prime::Generator;

const MIN_BITS: u64 = 64;
const MAX_BITS: u64 = 8192;

#[derive(Clone, Copy)]
struct AppState {
    generator: Generator,
    default_bits: u64,
}

#[derive(Deserialize)]
struct PrimeQuery {
    bits: Option<String>,
}

pub async fn run(config: &Config) -> io::Result<()> {
    let state = AppState {
        generator: Generator::new(config.rounds),
        default_bits: config.bits,
    };
    let app = Router::new()
        .route("/prime", get(prime))
        .route("/stats", get(stats))
        .route("/health", get(|| async { "ok" }))
        .with_state(state);
    let listener = tokio::net::TcpListener::bind(("0.0.0.0", config.port)).await?;
    println!("rust {LANG_VERSION} listening on :{}", config.port);
    axum::serve(listener, app).await
}

async fn prime(State(state): State<AppState>, Query(query): Query<PrimeQuery>) -> Response {
    let Some(bits) = parse_bits(query.bits.as_deref(), state.default_bits) else {
        let message = format!("bits must be an integer between {MIN_BITS} and {MAX_BITS}");
        return (StatusCode::BAD_REQUEST, message).into_response();
    };
    let (tx, rx) = oneshot::channel();
    rayon::spawn(move || {
        let start = Instant::now();
        let prime = state.generator.probable_prime(bits);
        let _ = tx.send((prime, start.elapsed()));
    });
    match rx.await {
        Ok((prime, elapsed)) => {
            let ms = elapsed.as_secs_f64() * 1000.0;
            Json(json!({ "bits": bits, "prime": prime.to_string(), "ms": ms })).into_response()
        }
        Err(_) => StatusCode::INTERNAL_SERVER_ERROR.into_response(),
    }
}

async fn stats() -> Json<serde_json::Value> {
    let stats = ProcessStats::read();
    Json(json!({
        "lang": LANG, "version": LANG_VERSION, "cpus": cpus(),
        "rss_mb": stats.rss_mb, "cpu_seconds": stats.cpu_seconds,
    }))
}

fn parse_bits(raw: Option<&str>, default_bits: u64) -> Option<u64> {
    match raw {
        None => Some(default_bits),
        Some(raw) => raw
            .parse()
            .ok()
            .filter(|bits| (MIN_BITS..=MAX_BITS).contains(bits)),
    }
}

#[cfg(test)]
mod tests {
    use super::parse_bits;

    #[test]
    fn parses_bits_within_range() {
        assert_eq!(parse_bits(None, 1024), Some(1024));
        assert_eq!(parse_bits(Some("2048"), 1024), Some(2048));
        assert_eq!(parse_bits(Some("32"), 1024), None);
        assert_eq!(parse_bits(Some("9000"), 1024), None);
        assert_eq!(parse_bits(Some("abc"), 1024), None);
        assert_eq!(parse_bits(Some(""), 1024), None);
    }
}
