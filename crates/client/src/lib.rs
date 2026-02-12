use anyhow::{Context, Result, bail};
use reqwest::header::{HeaderMap, HeaderValue};
use serde::Deserialize;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ItemCounter(pub u64);

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ItemId(pub u64);

#[derive(Debug, Deserialize)]
struct ApiEnvelope<T> {
    err: u8,
    result: Option<T>,
    message: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct Item {
    pub id: u64,
    pub project_id: u64,
    pub counter: u64,
    pub title: String,
    pub status: String,
    pub environment: Option<String>,
    pub total_occurrances: Option<u64>,
}

#[derive(Debug, Deserialize)]
#[serde(untagged)]
enum ItemByCounterResult {
    Item(Item),
    Redirect {
        #[serde(rename = "itemId")]
        item_id: u64,
        path: String,
        uri: String,
    },
}

pub struct RollbarClient {
    http: reqwest::Client,
    base_url: String,
}

impl RollbarClient {
    pub fn new(access_token: &str) -> Result<Self> {
        let mut headers = HeaderMap::new();

        headers.insert(
            "X-Rollbar-Access-Token",
            HeaderValue::from_str(access_token)?,
        );

        let http = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;

        Ok(Self {
            http,
            base_url: "https://api.rollbar.com/api/1".to_string(),
        })
    }

    pub async fn resolve_item_id_by_counter(&self, counter: ItemCounter) -> Result<ItemId> {
        let url = format!("{}/item_by_counter/{}", self.base_url, counter.0);

        let envelope: ApiEnvelope<ItemByCounterResult> = self
            .http
            .get(url)
            .send()
            .await
            .context("request to item_by_counter failed")?
            .error_for_status()
            .context("item_by_counter returned non-success status")?
            .json()
            .await
            .context("failed to decode item_by_counter response")?;

        if envelope.err != 0 {
            bail!(
                "rollbar item_by_counter: {}",
                envelope
                    .message
                    .as_deref()
                    .unwrap_or("unknown error from Rollbar")
            );
        }

        let result = envelope
            .result
            .context("item_by_counter response r=missing result")?;

        let item_id = match result {
            ItemByCounterResult::Item(item) => item.id,
            ItemByCounterResult::Redirect { item_id, .. } => item_id,
        };

        Ok(ItemId(item_id))
    }
}
