use std::collections::HashMap;
use std::time::Duration;

use serde::de::DeserializeOwned;
use serde::{Deserialize, Serialize};

const TIMEOUT: Duration = Duration::from_secs(5);
const MAX_ACCOUNT_ID_LENGTH: usize = 64;

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Account {
    id: String,
    currency: String,
    balance_minor: i64,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Transaction {
    amount_minor: i64,
}

#[derive(Deserialize)]
pub struct TransactionList {
    transactions: Vec<Transaction>,
}

#[derive(Deserialize)]
pub struct Rates {
    rates: HashMap<String, f64>,
}

#[derive(Serialize, Debug, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct Aggregate {
    account_id: String,
    currency: String,
    balance_minor: i64,
    transactions: usize,
    spent_minor: i64,
    received_minor: i64,
    balance_eur_minor: i64,
}

#[derive(Clone)]
pub struct Gateway {
    client: reqwest::Client,
    base: String,
}

impl Gateway {
    pub fn new(base: &str) -> Self {
        let client = reqwest::Client::builder()
            .http1_only()
            .pool_max_idle_per_host(4096)
            .timeout(TIMEOUT)
            .build()
            .expect("http client");
        Self {
            client,
            base: base.trim_end_matches('/').to_string(),
        }
    }

    pub async fn aggregate(&self, account_id: &str) -> Result<Aggregate, reqwest::Error> {
        let account_path = format!("/accounts/{account_id}");
        let transactions_path = format!("/accounts/{account_id}/transactions");
        let (account, transactions, rates) = tokio::try_join!(
            self.get::<Account>(&account_path),
            self.get::<TransactionList>(&transactions_path),
            self.get::<Rates>("/rates"),
        )?;
        Ok(summarize(account, transactions, rates))
    }

    async fn get<T: DeserializeOwned>(&self, path: &str) -> Result<T, reqwest::Error> {
        self.client
            .get(format!("{}{path}", self.base))
            .send()
            .await?
            .error_for_status()?
            .json()
            .await
    }
}

pub fn summarize(account: Account, list: TransactionList, rates: Rates) -> Aggregate {
    let spent_minor = list
        .transactions
        .iter()
        .filter(|t| t.amount_minor < 0)
        .map(|t| -t.amount_minor)
        .sum();
    let received_minor = list
        .transactions
        .iter()
        .filter(|t| t.amount_minor >= 0)
        .map(|t| t.amount_minor)
        .sum();
    let eur = rates.rates.get("EUR").copied().unwrap_or(0.0);
    Aggregate {
        account_id: account.id,
        currency: account.currency,
        balance_minor: account.balance_minor,
        transactions: list.transactions.len(),
        spent_minor,
        received_minor,
        balance_eur_minor: (account.balance_minor as f64 * eur).round() as i64,
    }
}

pub fn valid_account_id(id: &str) -> bool {
    !id.is_empty()
        && id.len() <= MAX_ACCOUNT_ID_LENGTH
        && id.chars().all(|c| c == '-' || c.is_ascii_alphanumeric())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::test_support::{ACCOUNT, RATES, TRANSACTIONS, stub};

    fn expected() -> Aggregate {
        Aggregate {
            account_id: "acc-7".into(),
            currency: "USD".into(),
            balance_minor: 1250075,
            transactions: 3,
            spent_minor: 2000,
            received_minor: 650000,
            balance_eur_minor: 1150069,
        }
    }

    #[test]
    fn summarize_totals_spent_received_and_converts_balance() {
        let aggregate = summarize(
            serde_json::from_str(ACCOUNT).unwrap(),
            serde_json::from_str(TRANSACTIONS).unwrap(),
            serde_json::from_str(RATES).unwrap(),
        );
        assert_eq!(aggregate, expected());
    }

    #[tokio::test]
    async fn aggregate_calls_all_three_downstream_endpoints() {
        let url = stub(vec![
            ("/accounts/acc-7", ACCOUNT),
            ("/accounts/acc-7/transactions", TRANSACTIONS),
            ("/rates", RATES),
        ])
        .await;
        assert_eq!(
            Gateway::new(&url).aggregate("acc-7").await.unwrap(),
            expected()
        );
    }

    #[tokio::test]
    async fn aggregate_fails_when_a_downstream_call_fails() {
        let url = stub(vec![("/rates", RATES)]).await;
        assert!(Gateway::new(&url).aggregate("acc-7").await.is_err());
    }

    #[test]
    fn validates_account_ids() {
        for (id, expected) in [
            ("acc-7", true),
            ("ACC-123", true),
            ("", false),
            ("acc/7", false),
            ("../rates", false),
            ("acc 7", false),
        ] {
            assert_eq!(valid_account_id(id), expected, "{id}");
        }
    }

    #[tokio::test]
    async fn aggregate_fails_when_downstream_json_misses_required_fields() {
        for transactions in [
            r#"{"accountId":"acc-7"}"#,
            r#"{"accountId":"acc-7","transactions":null}"#,
            r#"{"transactions":[{"id":"tx-1"}]}"#,
        ] {
            let url = stub(vec![
                ("/accounts/acc-7", ACCOUNT),
                ("/accounts/acc-7/transactions", transactions),
                ("/rates", RATES),
            ])
            .await;
            assert!(
                Gateway::new(&url).aggregate("acc-7").await.is_err(),
                "{transactions}"
            );
        }
    }
}
