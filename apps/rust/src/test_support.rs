pub const ACCOUNT: &str = r#"{"id":"acc-7","owner":"Ada Example","currency":"USD","balanceMinor":1250075,"status":"active"}"#;
pub const TRANSACTIONS: &str = r#"{"accountId":"acc-7","transactions":[{"id":"tx-1","amountMinor":-1250,"category":"groceries"},{"id":"tx-2","amountMinor":650000,"category":"salary"},{"id":"tx-3","amountMinor":-750,"category":"transport"}]}"#;
pub const RATES: &str = r#"{"base":"USD","rates":{"EUR":0.92,"GBP":0.79}}"#;

pub async fn stub(routes: Vec<(&'static str, &'static str)>) -> String {
    let mut app = axum::Router::new();
    for (path, body) in routes {
        app = app.route(path, axum::routing::get(move || async move { body }));
    }
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let url = format!("http://{}", listener.local_addr().unwrap());
    tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
    url
}
