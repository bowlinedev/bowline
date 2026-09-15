use crate::error::decode_remote;
use crate::sse;
use crate::{Code, Error, Issue, RemoteError};
use bytes::Bytes;
use futures_core::Stream;
use futures_util::StreamExt;
use reqwest::header::{HeaderMap, HeaderValue, ACCEPT, CONTENT_TYPE};
use serde::de::DeserializeOwned;
use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, VecDeque};
use std::pin::Pin;
use std::sync::Arc;
use std::time::Duration;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Method {
    Get,
    Post,
}

#[derive(Clone, Debug, Default)]
pub struct CallOptions {
    pub headers: HeaderMap,
    pub timeout: Option<Duration>,
}

impl CallOptions {
    pub fn new() -> Self {
        CallOptions::default()
    }

    pub fn header(mut self, name: &'static str, value: &str) -> Self {
        if let Ok(v) = HeaderValue::from_str(value) {
            self.headers.insert(name, v);
        }
        self
    }

    pub fn timeout(mut self, d: Duration) -> Self {
        self.timeout = Some(d);
        self
    }
}

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct Empty {}

#[derive(
    Clone, Copy, Debug, Default, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize,
)]
#[serde(transparent)]
pub struct DurationNs(pub i64);

impl DurationNs {
    pub fn as_duration(&self) -> Duration {
        Duration::from_nanos(self.0.max(0) as u64)
    }
}

#[derive(
    Clone, Copy, Debug, Default, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize,
)]
#[serde(transparent)]
pub struct StringInt(#[serde(with = "crate::codec::string_int")] pub i64);

#[derive(
    Clone, Copy, Debug, Default, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize,
)]
#[serde(transparent)]
pub struct StringUint(#[serde(with = "crate::codec::string_uint")] pub u64);

#[derive(Clone, Debug, Default, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct Base64Bytes(#[serde(with = "crate::codec::base64")] pub Vec<u8>);

pub trait Validate {
    fn validate(&self, path: &mut Vec<String>, issues: &mut Vec<Issue>);

    fn issues(&self) -> Vec<Issue> {
        let mut path = Vec::new();
        let mut issues = Vec::new();
        self.validate(&mut path, &mut issues);
        issues
    }
}

macro_rules! leaf_validate {
    ($($t:ty),* $(,)?) => {
        $(impl Validate for $t {
            fn validate(&self, _path: &mut Vec<String>, _issues: &mut Vec<Issue>) {}
        })*
    };
}

leaf_validate!(
    String,
    str,
    bool,
    i8,
    i16,
    i32,
    i64,
    u8,
    u16,
    u32,
    u64,
    f32,
    f64,
    serde_json::Value,
    chrono::DateTime<chrono::Utc>,
    Empty,
    DurationNs,
    StringInt,
    StringUint,
    Base64Bytes,
);

impl<T: Validate> Validate for Vec<T> {
    fn validate(&self, path: &mut Vec<String>, issues: &mut Vec<Issue>) {
        for (i, item) in self.iter().enumerate() {
            path.push(i.to_string());
            item.validate(path, issues);
            path.pop();
        }
    }
}

impl<T: Validate, const N: usize> Validate for [T; N] {
    fn validate(&self, path: &mut Vec<String>, issues: &mut Vec<Issue>) {
        for (i, item) in self.iter().enumerate() {
            path.push(i.to_string());
            item.validate(path, issues);
            path.pop();
        }
    }
}

impl<T: Validate> Validate for Option<T> {
    fn validate(&self, path: &mut Vec<String>, issues: &mut Vec<Issue>) {
        if let Some(v) = self {
            v.validate(path, issues);
        }
    }
}

impl<T: Validate> Validate for Box<T> {
    fn validate(&self, path: &mut Vec<String>, issues: &mut Vec<Issue>) {
        (**self).validate(path, issues);
    }
}

impl<K: ToString, V: Validate> Validate for BTreeMap<K, V> {
    fn validate(&self, path: &mut Vec<String>, issues: &mut Vec<Issue>) {
        for (key, item) in self {
            path.push(key.to_string());
            item.validate(path, issues);
            path.pop();
        }
    }
}

type HeaderSource = Arc<dyn Fn() -> HeaderMap + Send + Sync>;

#[derive(Clone)]
pub struct Transport {
    base: String,
    client: reqwest::Client,
    headers: Option<HeaderSource>,
}

impl std::fmt::Debug for Transport {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Transport")
            .field("base", &self.base)
            .finish()
    }
}

impl Transport {
    pub fn new(base_url: impl Into<String>) -> Self {
        Transport::with_client(base_url, reqwest::Client::new())
    }

    pub fn with_client(base_url: impl Into<String>, client: reqwest::Client) -> Self {
        let base = base_url.into();
        Transport {
            base: base.trim_end_matches('/').to_string(),
            client,
            headers: None,
        }
    }

    pub fn with_headers(mut self, f: impl Fn() -> HeaderMap + Send + Sync + 'static) -> Self {
        self.headers = Some(Arc::new(f));
        self
    }

    pub fn base_url(&self) -> &str {
        &self.base
    }

    fn prepare<I: Serialize + Validate>(
        &self,
        path: &str,
        method: Method,
        input: &I,
        options: Option<&CallOptions>,
        accept: &'static str,
    ) -> Result<reqwest::RequestBuilder, Error> {
        let issues = input.issues();
        if !issues.is_empty() {
            return Err(Error::Invalid(issues));
        }
        let body = serde_json::to_string(input)?;
        let url = format!("{}/{}", self.base, path);
        let mut request = match method {
            Method::Get => self.client.get(url).query(&[("input", body.as_str())]),
            Method::Post => self
                .client
                .post(url)
                .header(CONTENT_TYPE, "application/json")
                .body(body),
        };
        request = request.header(ACCEPT, accept);
        Ok(self.apply(request, options))
    }

    fn apply(
        &self,
        mut request: reqwest::RequestBuilder,
        options: Option<&CallOptions>,
    ) -> reqwest::RequestBuilder {
        if let Some(source) = &self.headers {
            request = request.headers(source());
        }
        if let Some(options) = options {
            request = request.headers(options.headers.clone());
            if let Some(timeout) = options.timeout {
                request = request.timeout(timeout);
            }
        }
        request
    }

    async fn send(request: reqwest::RequestBuilder) -> Result<reqwest::Response, Error> {
        let response = request.send().await.map_err(map_transport)?;
        let status = response.status();
        if status.as_u16() >= 400 {
            let body = response.bytes().await.map_err(map_transport)?;
            return Err(Error::Remote(decode_remote(status.as_u16(), &body)));
        }
        Ok(response)
    }

    pub async fn call<I: Serialize + Validate, O: DeserializeOwned>(
        &self,
        path: &str,
        method: Method,
        input: &I,
        options: Option<&CallOptions>,
    ) -> Result<O, Error> {
        let request = self.prepare(path, method, input, options, "application/json")?;
        let response = Transport::send(request).await?;
        let body = response.bytes().await.map_err(map_transport)?;
        decode_body(&body)
    }

    pub fn subscribe<I: Serialize + Validate, O: DeserializeOwned + Send + 'static>(
        &self,
        path: &str,
        method: Method,
        input: &I,
        options: Option<&CallOptions>,
    ) -> impl Stream<Item = Result<O, Error>> + Send + 'static {
        let prepared = self.prepare(path, method, input, options, "text/event-stream");
        futures_util::stream::unfold(SubscribeState::Start(prepared), |state| async move {
            let mut state = state;
            loop {
                match state {
                    SubscribeState::Start(Err(err)) => {
                        return Some((Err(err), SubscribeState::Done))
                    }
                    SubscribeState::Start(Ok(request)) => match Transport::send(request).await {
                        Ok(response) => {
                            state = SubscribeState::Reading {
                                body: Some(Box::pin(response.bytes_stream())),
                                parser: sse::Parser::new(),
                                queue: VecDeque::new(),
                            };
                        }
                        Err(err) => return Some((Err(err), SubscribeState::Done)),
                    },
                    SubscribeState::Reading {
                        mut body,
                        mut parser,
                        mut queue,
                    } => {
                        if let Some(item) = queue.pop_front() {
                            let next = if item.is_err() {
                                SubscribeState::Done
                            } else {
                                SubscribeState::Reading {
                                    body,
                                    parser,
                                    queue,
                                }
                            };
                            return Some((item, next));
                        }
                        let stream = body.as_mut()?;
                        match stream.next().await {
                            Some(Ok(chunk)) => {
                                for event in parser.push(&chunk) {
                                    match shape_event::<O>(&event) {
                                        Some(item) => queue.push_back(item),
                                        None => {
                                            body = None;
                                            break;
                                        }
                                    }
                                }
                            }
                            Some(Err(err)) => {
                                queue.push_back(Err(map_transport(err)));
                                body = None;
                            }
                            None => {
                                if let Some(event) = parser.finish() {
                                    if let Some(item) = shape_event::<O>(&event) {
                                        queue.push_back(item);
                                    }
                                }
                                body = None;
                            }
                        }
                        state = SubscribeState::Reading {
                            body,
                            parser,
                            queue,
                        };
                    }
                    SubscribeState::Done => return None,
                }
            }
        })
    }

    pub async fn upload<I: Serialize + Validate, O: DeserializeOwned>(
        &self,
        path: &str,
        input: &I,
        file: reqwest::Body,
        filename: &str,
        options: Option<&CallOptions>,
    ) -> Result<O, Error> {
        let issues = input.issues();
        if !issues.is_empty() {
            return Err(Error::Invalid(issues));
        }
        let body = serde_json::to_string(input)?;
        let input_part = reqwest::multipart::Part::text(body)
            .mime_str("application/json")
            .map_err(map_transport)?;
        let file_part = reqwest::multipart::Part::stream(file).file_name(filename.to_string());
        let form = reqwest::multipart::Form::new()
            .part("input", input_part)
            .part("file", file_part);
        let request = self
            .client
            .post(format!("{}/{}", self.base, path))
            .header(ACCEPT, "application/json")
            .multipart(form);
        let response = Transport::send(self.apply(request, options)).await?;
        let bytes = response.bytes().await.map_err(map_transport)?;
        decode_body(&bytes)
    }
}

type ByteStream = Pin<Box<dyn Stream<Item = Result<Bytes, reqwest::Error>> + Send>>;

enum SubscribeState<O> {
    Start(Result<reqwest::RequestBuilder, Error>),
    Reading {
        body: Option<ByteStream>,
        parser: sse::Parser,
        queue: VecDeque<Result<O, Error>>,
    },
    Done,
}

fn shape_event<O: DeserializeOwned>(event: &sse::Event) -> Option<Result<O, Error>> {
    match event.name.as_str() {
        "message" => Some(serde_json::from_str(&event.data).map_err(Error::Decode)),
        "error" => Some(Err(Error::Remote(decode_remote(
            500,
            event.data.as_bytes(),
        )))),
        "done" => None,
        _ => Some(Err(Error::Remote(RemoteError::new(
            Code::Internal,
            format!("unexpected event {}", event.name),
            0,
        )))),
    }
}

fn decode_body<O: DeserializeOwned>(body: &[u8]) -> Result<O, Error> {
    if body.iter().all(|b| b.is_ascii_whitespace()) {
        return serde_json::from_str("{}").map_err(Error::Decode);
    }
    serde_json::from_slice(body).map_err(Error::Decode)
}

fn map_transport(err: reqwest::Error) -> Error {
    if err.is_timeout() {
        return Error::Remote(RemoteError::new(Code::DeadlineExceeded, err.to_string(), 0));
    }
    Error::Transport(err)
}
