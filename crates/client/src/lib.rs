use anyhow::{Context, Result, bail};
use reqwest::header::{HeaderMap, HeaderValue};
use serde::{Deserialize, de::DeserializeOwned};
use serde_json::Value;

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
    pub total_occurrences: Option<u64>,
}

#[derive(Debug, Deserialize)]
#[serde(untagged)]
enum ItemByCounterResult {
    Item(Item),
    Redirect {
        #[serde(rename = "itemId")]
        item_id: u64,
    },
}

#[derive(Debug, Deserialize)]
pub struct ItemInstance {
    pub id: u64,
    #[serde(default)]
    pub timestamp: Option<u64>,
    #[serde(default)]
    pub body: Option<Value>,
    #[serde(default)]
    pub data: Option<Value>,
}

#[derive(Debug, Deserialize)]
#[serde(untagged)]
enum ItemInstanceResult {
    List(Vec<ItemInstance>),
    Wrapped { instances: Vec<ItemInstance> },
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

    async fn get_result<T>(&self, path: &str, op: &str) -> Result<T>
    where
        T: DeserializeOwned,
    {
        let url = format!("{}{}", self.base_url, path);

        let envelope: ApiEnvelope<T> = self
            .http
            .get(url)
            .send()
            .await
            .with_context(|| format!("request to {op} failed"))?
            .error_for_status()
            .with_context(|| format!("{op} returned non-success status"))?
            .json()
            .await
            .with_context(|| format!("failed to decode {op} response"))?;

        if envelope.err != 0 {
            bail!(
                "rollbar {op}: {}",
                envelope
                    .message
                    .as_deref()
                    .unwrap_or("unknown error from Rollbar")
            );
        }

        envelope
            .result
            .with_context(|| format!("{op} response is missing result"))
    }

    pub async fn resolve_item_id_by_counter(&self, counter: ItemCounter) -> Result<ItemId> {
        let path = format!("/item_by_counter/{}", counter.0);

        let result: ItemByCounterResult = self.get_result(&path, "item_by_counter").await?;

        let item_id = match result {
            ItemByCounterResult::Item(item) => item.id,
            ItemByCounterResult::Redirect { item_id, .. } => item_id,
        };

        Ok(ItemId(item_id))
    }

    pub async fn get_item(&self, item_id: ItemId) -> Result<Item> {
        let path = format!("/item/{}/", item_id.0);

        self.get_result(&path, "item").await
    }

    pub async fn get_latest_instance(&self, item_id: ItemId) -> Result<Option<ItemInstance>> {
        let path = format!("/item/{}/instances?per_page=1", item_id.0);

        let result: ItemInstanceResult = self.get_result(&path, "item instances").await?;
        let mut instances = match result {
            ItemInstanceResult::List(v) => v,
            ItemInstanceResult::Wrapped { instances } => instances,
        };

        Ok(instances.pop())
    }
}
