use serde::de::DeserializeOwned;
use serde::{Deserialize, Serialize};
use std::fmt;

#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum Code {
    Canceled,
    InvalidArgument,
    DeadlineExceeded,
    NotFound,
    AlreadyExists,
    PermissionDenied,
    ResourceExhausted,
    FailedPrecondition,
    Aborted,
    OutOfRange,
    Unimplemented,
    Internal,
    Unavailable,
    DataLoss,
    Unauthenticated,
    #[serde(other)]
    Unknown,
}

impl Code {
    pub fn as_str(&self) -> &'static str {
        match self {
            Code::Canceled => "CANCELED",
            Code::Unknown => "UNKNOWN",
            Code::InvalidArgument => "INVALID_ARGUMENT",
            Code::DeadlineExceeded => "DEADLINE_EXCEEDED",
            Code::NotFound => "NOT_FOUND",
            Code::AlreadyExists => "ALREADY_EXISTS",
            Code::PermissionDenied => "PERMISSION_DENIED",
            Code::ResourceExhausted => "RESOURCE_EXHAUSTED",
            Code::FailedPrecondition => "FAILED_PRECONDITION",
            Code::Aborted => "ABORTED",
            Code::OutOfRange => "OUT_OF_RANGE",
            Code::Unimplemented => "UNIMPLEMENTED",
            Code::Internal => "INTERNAL",
            Code::Unavailable => "UNAVAILABLE",
            Code::DataLoss => "DATA_LOSS",
            Code::Unauthenticated => "UNAUTHENTICATED",
        }
    }

    pub fn http_status(&self) -> u16 {
        match self {
            Code::Canceled | Code::DeadlineExceeded => 408,
            Code::Unknown | Code::Internal | Code::DataLoss => 500,
            Code::InvalidArgument | Code::OutOfRange => 400,
            Code::NotFound | Code::Unimplemented => 404,
            Code::AlreadyExists | Code::Aborted => 409,
            Code::PermissionDenied => 403,
            Code::ResourceExhausted => 429,
            Code::FailedPrecondition => 412,
            Code::Unavailable => 503,
            Code::Unauthenticated => 401,
        }
    }
}

impl fmt::Display for Code {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct Issue {
    pub path: Vec<String>,
    pub rule: String,
    pub message: String,
}

impl Issue {
    pub fn new(path: Vec<String>, rule: &str, message: impl Into<String>) -> Self {
        Issue {
            path,
            rule: rule.to_string(),
            message: message.into(),
        }
    }
}

impl fmt::Display for Issue {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.path.join("."), self.message)
    }
}

#[derive(Clone, Debug, PartialEq)]
pub struct RemoteError {
    pub code: Code,
    pub message: String,
    pub status: u16,
    pub variant: Option<String>,
    pub details: Option<serde_json::Value>,
    pub issues: Vec<Issue>,
}

impl RemoteError {
    pub fn new(code: Code, message: impl Into<String>, status: u16) -> Self {
        RemoteError {
            code,
            message: message.into(),
            status,
            variant: None,
            details: None,
            issues: Vec::new(),
        }
    }

    pub fn variant(&self) -> Option<&str> {
        self.variant.as_deref()
    }

    pub fn details_as<T: DeserializeOwned>(&self) -> Option<T> {
        let details = self.details.clone()?;
        serde_json::from_value(details).ok()
    }
}

impl fmt::Display for RemoteError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.code, self.message)?;
        for issue in &self.issues {
            write!(f, "; {issue}")?;
        }
        Ok(())
    }
}

#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("{0}")]
    Remote(RemoteError),
    #[error("transport error: {0}")]
    Transport(#[from] reqwest::Error),
    #[error("decode error: {0}")]
    Decode(#[from] serde_json::Error),
    #[error("invalid input: {}", format_issues(.0))]
    Invalid(Vec<Issue>),
    #[error("request canceled")]
    Canceled,
}

fn format_issues(issues: &[Issue]) -> String {
    issues
        .iter()
        .map(|i| i.to_string())
        .collect::<Vec<_>>()
        .join("; ")
}

impl Error {
    pub fn code(&self) -> Option<Code> {
        match self {
            Error::Remote(e) => Some(e.code),
            Error::Canceled => Some(Code::Canceled),
            Error::Invalid(_) => Some(Code::InvalidArgument),
            _ => None,
        }
    }

    pub fn remote(&self) -> Option<&RemoteError> {
        match self {
            Error::Remote(e) => Some(e),
            _ => None,
        }
    }

    pub fn variant(&self) -> Option<&str> {
        self.remote().and_then(|e| e.variant())
    }

    pub fn details_as<T: DeserializeOwned>(&self) -> Option<T> {
        self.remote().and_then(|e| e.details_as())
    }
}

#[derive(Deserialize)]
struct WireError {
    code: Code,
    message: String,
    #[serde(rename = "type")]
    variant: Option<String>,
    details: Option<serde_json::Value>,
    #[serde(default)]
    issues: Vec<Issue>,
}

#[derive(Deserialize)]
struct WireEnvelope {
    error: WireError,
}

pub(crate) fn decode_remote(status: u16, body: &[u8]) -> RemoteError {
    match serde_json::from_slice::<WireEnvelope>(body) {
        Ok(env) => RemoteError {
            code: env.error.code,
            message: env.error.message,
            status,
            variant: env.error.variant,
            details: env.error.details,
            issues: env.error.issues,
        },
        Err(_) => RemoteError::new(Code::Unknown, format!("HTTP {status}"), status),
    }
}
