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
use crate::gateway::{Gateway, valid_account_id};
use crate::metrics::{LANG, LANG_VERSION, ProcessStats, cpus};
use crate::prime::Generator;

const MIN_BITS: u64 = 64;
const MAX_BITS: u64 = 8192;

#[derive(Clone)]
struct AppState {
    generator: Generator,
    default_bits: u64,
    gateway: Gateway,
}

#[derive(Deserialize)]
struct PrimeQuery {
    bits: Option<String>,
}

#[derive(Deserialize)]
struct AggregateQuery {
    account: Option<String>,
}

pub async fn run(config: &Config) -> io::Result<()> {
    let app = router(config.rounds, config.bits, &config.downstream_url);
    let listener = tokio::net::TcpListener::bind(("0.0.0.0", config.port)).await?;
    println!("rust {LANG_VERSION} listening on :{}", config.port);
    axum::serve(listener, app).await
}

fn router(rounds: u32, default_bits: u64, downstream_url: &str) -> Router {
    let state = AppState {
        generator: Generator::new(rounds),
        default_bits,
        gateway: Gateway::new(downstream_url),
    };
    Router::new()
        .route("/prime", get(prime))
        .route("/aggregate", get(aggregate))
        .route("/stats", get(stats))
        .route("/health", get(|| async { "ok" }))
        .with_state(state)
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

async fn aggregate(State(state): State<AppState>, Query(query): Query<AggregateQuery>) -> Response {
    let Some(account_id) = query.account.filter(|id| valid_account_id(id)) else {
        return (
            StatusCode::BAD_REQUEST,
            "account must be 1 to 64 letters, digits or dashes",
        )
            .into_response();
    };
    match state.gateway.aggregate(&account_id).await {
        Ok(result) => Json(result).into_response(),
        Err(error) => {
            eprintln!("downstream call failed: {}", error.without_url());
            (StatusCode::BAD_GATEWAY, "downstream unavailable").into_response()
        }
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

    use super::router;
    use crate::test_support::{ACCOUNT, RATES, TRANSACTIONS, stub};

    #[tokio::test]
    async fn endpoints_answer_with_the_expected_status() {
        let downstream = stub(vec![
            ("/accounts/acc-7", ACCOUNT),
            ("/accounts/acc-7/transactions", TRANSACTIONS),
            ("/rates", RATES),
        ])
        .await;
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let base = format!("http://{}", listener.local_addr().unwrap());
        tokio::spawn(async move {
            axum::serve(listener, router(5, 64, &downstream))
                .await
                .unwrap()
        });

        for (path, expected) in [
            ("/prime", 200),
            ("/prime?bits=128", 200),
            ("/prime?bits=1", 400),
            ("/aggregate?account=acc-7", 200),
            ("/aggregate?account=acc-8", 502),
            ("/aggregate?account=acc%2F7", 400),
            ("/aggregate", 400),
            ("/stats", 200),
            ("/health", 200),
        ] {
            let status = reqwest::get(format!("{base}{path}"))
                .await
                .unwrap()
                .status();
            assert_eq!(status.as_u16(), expected, "{path}");
        }
        let body = reqwest::get(format!("{base}/aggregate?account=acc-8"))
            .await
            .unwrap()
            .text()
            .await
            .unwrap();
        assert_eq!(body, "downstream unavailable");
        let aggregate = reqwest::get(format!("{base}/aggregate?account=acc-7"))
            .await
            .unwrap()
            .text()
            .await
            .unwrap();
        assert!(
            aggregate.contains(r#""balanceEurMinor":1150069"#),
            "{aggregate}"
        );
    }
}
