use bowline_client::{
    rules, Base64Bytes, CallOptions, Code, Empty, Error, Issue, Method, StringInt, Transport,
    Validate,
};
use futures::StreamExt;
use reqwest::header::HeaderMap;
use serde::{Deserialize, Serialize};
use std::time::Duration;
use wiremock::matchers::{body_string_contains, header, method, path, query_param};
use wiremock::{Mock, MockServer, Request, ResponseTemplate};

#[derive(Serialize)]
struct GetInput {
    id: i64,
}

impl Validate for GetInput {
    fn validate(&self, path: &mut Vec<String>, issues: &mut Vec<Issue>) {
        rules::required_int(self.id, path, "id", issues);
    }
}

#[derive(Debug, Deserialize, PartialEq)]
struct User {
    id: i64,
    name: String,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
struct Encoded {
    #[serde(with = "bowline_client::codec::string_int")]
    big: i64,
    #[serde(with = "bowline_client::codec::base64")]
    blob: Vec<u8>,
    nested: Vec<StringInt>,
    raw: Base64Bytes,
    at: chrono::DateTime<chrono::Utc>,
}

impl Validate for Encoded {
    fn validate(&self, _path: &mut Vec<String>, _issues: &mut Vec<Issue>) {}
}

fn transport(server: &MockServer) -> Transport {
    Transport::new(format!("{}/api/", server.uri()))
}

#[tokio::test]
async fn get_sends_the_input_query_parameter_and_decodes_the_body() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/users.get"))
        .and(query_param("input", "{\"id\":3}"))
        .and(header("accept", "application/json"))
        .respond_with(ResponseTemplate::new(200).set_body_string("{\"id\":3,\"name\":\"Ada\"}"))
        .mount(&server)
        .await;
    let user: User = transport(&server)
        .call("users.get", Method::Get, &GetInput { id: 3 }, None)
        .await
        .unwrap();
    assert_eq!(
        user,
        User {
            id: 3,
            name: "Ada".into()
        }
    );
}

#[tokio::test]
async fn post_sends_a_json_body_with_merged_headers() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/users.rename"))
        .and(header("content-type", "application/json"))
        .and(header("authorization", "Bearer per-call"))
        .and(header("x-tenant", "acme"))
        .and(body_string_contains("\"id\":3"))
        .respond_with(ResponseTemplate::new(200).set_body_string("{\"id\":3,\"name\":\"Grace\"}"))
        .mount(&server)
        .await;
    let transport = transport(&server).with_headers(|| {
        let mut h = HeaderMap::new();
        h.insert("authorization", "Bearer static".parse().unwrap());
        h.insert("x-tenant", "acme".parse().unwrap());
        h
    });
    let options = CallOptions::new().header("authorization", "Bearer per-call");
    let user: User = transport
        .call(
            "users.rename",
            Method::Post,
            &GetInput { id: 3 },
            Some(&options),
        )
        .await
        .unwrap();
    assert_eq!(user.name, "Grace");
}

#[tokio::test]
async fn envelope_errors_become_remote_errors() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/users.get"))
        .respond_with(ResponseTemplate::new(412).set_body_string(
            "{\"error\":{\"code\":\"FAILED_PRECONDITION\",\"message\":\"locked\",\"type\":\"Locked\",\"details\":{\"id\":3},\"issues\":[{\"path\":[\"id\"],\"rule\":\"exists\",\"message\":\"gone\"}]}}",
        ))
        .mount(&server)
        .await;
    let err = transport(&server)
        .call::<_, User>("users.get", Method::Get, &GetInput { id: 3 }, None)
        .await
        .unwrap_err();
    let remote = err.remote().expect("remote error");
    assert_eq!(remote.code, Code::FailedPrecondition);
    assert_eq!(remote.status, 412);
    assert_eq!(remote.message, "locked");
    assert_eq!(remote.variant(), Some("Locked"));
    assert_eq!(remote.issues.len(), 1);
    #[derive(Deserialize)]
    struct Details {
        id: i64,
    }
    assert_eq!(err.details_as::<Details>().map(|d| d.id), Some(3));
    assert_eq!(err.code(), Some(Code::FailedPrecondition));
    assert_eq!(err.to_string(), "FAILED_PRECONDITION: locked; id: gone");
}

#[tokio::test]
async fn non_envelope_failures_map_to_unknown_and_unknown_codes_survive() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/users.get"))
        .respond_with(ResponseTemplate::new(502).set_body_string("<html>bad gateway</html>"))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/api/users.new"))
        .respond_with(
            ResponseTemplate::new(418)
                .set_body_string("{\"error\":{\"code\":\"TEAPOT\",\"message\":\"short\"}}"),
        )
        .mount(&server)
        .await;
    let t = transport(&server);
    let err = t
        .call::<_, User>("users.get", Method::Get, &GetInput { id: 3 }, None)
        .await
        .unwrap_err();
    let remote = err.remote().unwrap();
    assert_eq!((remote.code, remote.status), (Code::Unknown, 502));
    let err = t
        .call::<_, User>("users.new", Method::Get, &GetInput { id: 3 }, None)
        .await
        .unwrap_err();
    assert_eq!(err.remote().unwrap().code, Code::Unknown);
    assert_eq!(err.remote().unwrap().message, "short");
}

#[tokio::test]
async fn transport_failures_and_timeouts() {
    let err = Transport::new("http://127.0.0.1:9")
        .call::<_, User>("users.get", Method::Get, &GetInput { id: 3 }, None)
        .await
        .unwrap_err();
    assert!(matches!(err, Error::Transport(_)));
    assert_eq!(err.code(), None);

    let slow = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/users.get"))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_string("{\"id\":1,\"name\":\"x\"}")
                .set_delay(Duration::from_millis(400)),
        )
        .mount(&slow)
        .await;
    let options = CallOptions::new().timeout(Duration::from_millis(50));
    let err = transport(&slow)
        .call::<_, User>(
            "users.get",
            Method::Get,
            &GetInput { id: 3 },
            Some(&options),
        )
        .await
        .unwrap_err();
    assert_eq!(err.code(), Some(Code::DeadlineExceeded));
}

#[tokio::test]
async fn invalid_input_short_circuits_before_any_request() {
    let server = MockServer::start().await;
    let err = transport(&server)
        .call::<_, User>("users.get", Method::Get, &GetInput { id: 0 }, None)
        .await
        .unwrap_err();
    match err {
        Error::Invalid(issues) => {
            assert_eq!(
                issues,
                vec![Issue::new(vec!["id".into()], "required", "is required")]
            );
        }
        other => panic!("unexpected {other:?}"),
    }
    assert!(server.received_requests().await.unwrap().is_empty());
}

#[tokio::test]
async fn string_ints_base64_and_timestamps_round_trip() {
    let json = "{\"big\":\"9007199254740993\",\"blob\":\"aGVsbG8=\",\"nested\":[\"1\",\"22\"],\"raw\":\"aGk=\",\"at\":\"2026-01-02T03:04:05Z\"}";
    let decoded: Encoded = serde_json::from_str(json).unwrap();
    assert_eq!(decoded.big, 9007199254740993);
    assert_eq!(decoded.blob, b"hello");
    assert_eq!(decoded.nested, vec![StringInt(1), StringInt(22)]);
    assert_eq!(decoded.raw.0, b"hi");
    assert_eq!(decoded.at.timestamp(), 1767323045);
    assert_eq!(serde_json::to_string(&decoded).unwrap(), json);
}

#[tokio::test]
async fn subscriptions_stream_messages_until_done_or_error() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/ticks"))
        .and(header("accept", "text/event-stream"))
        .respond_with(
            ResponseTemplate::new(200)
                .insert_header("content-type", "text/event-stream")
                .set_body_string(": open\n\nevent: message\ndata: {\"id\":1,\"name\":\"a\"}\n\n: ping\n\nevent: message\ndata: {\"id\":2,\"name\":\"b\"}\n\nevent: done\ndata: {}\n\nevent: message\ndata: {\"id\":9,\"name\":\"never\"}\n\n"),
        )
        .mount(&server)
        .await;
    Mock::given(method("POST"))
        .and(path("/api/fails"))
        .respond_with(
            ResponseTemplate::new(200)
                .insert_header("content-type", "text/event-stream")
                .set_body_string("event: message\ndata: {\"id\":1,\"name\":\"a\"}\n\nevent: error\ndata: {\"error\":{\"code\":\"NOT_FOUND\",\"message\":\"gone\"}}\n\n"),
        )
        .mount(&server)
        .await;
    let t = transport(&server);
    let items: Vec<Result<User, Error>> = t
        .subscribe("ticks", Method::Get, &Empty {}, None)
        .collect()
        .await;
    let names: Vec<String> = items.into_iter().map(|r| r.unwrap().name).collect();
    assert_eq!(names, vec!["a", "b"]);

    let mut failing = Box::pin(t.subscribe::<_, User>("fails", Method::Post, &Empty {}, None));
    assert_eq!(failing.next().await.unwrap().unwrap().id, 1);
    let err = failing.next().await.unwrap().unwrap_err();
    assert_eq!(err.code(), Some(Code::NotFound));
    assert!(failing.next().await.is_none());

    let rejected: Vec<Result<User, Error>> = t
        .subscribe("ticks", Method::Get, &GetInput { id: 0 }, None)
        .collect()
        .await;
    assert_eq!(rejected.len(), 1);
    assert!(matches!(rejected[0], Err(Error::Invalid(_))));
}

#[tokio::test]
async fn uploads_send_the_input_part_then_the_file_part() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/attach"))
        .respond_with(|req: &Request| {
            let body = String::from_utf8_lossy(&req.body).into_owned();
            let input = body.find("name=\"input\"").unwrap();
            let file = body.find("name=\"file\"").unwrap();
            assert!(input < file);
            assert!(body.contains("filename=\"receipt.bin\""));
            assert!(body.contains("Content-Type: application/json"));
            assert!(body.contains("hello upload"));
            ResponseTemplate::new(200).set_body_string("{\"id\":7,\"name\":\"receipt.bin\"}")
        })
        .mount(&server)
        .await;
    let user: User = transport(&server)
        .upload(
            "attach",
            &GetInput { id: 3 },
            reqwest::Body::from("hello upload"),
            "receipt.bin",
            None,
        )
        .await
        .unwrap();
    assert_eq!(user.name, "receipt.bin");
}

#[test]
fn rules_produce_the_runtime_messages() {
    let mut issues = Vec::new();
    let path = vec!["lines".to_string(), "0".to_string()];
    rules::required_str("", &path, "description", &mut issues);
    rules::min_len(
        rules::chars("ab"),
        3,
        " characters",
        &path,
        "description",
        &mut issues,
    );
    rules::max_len(4, 2, " items", &path, "tags", &mut issues);
    rules::exact_len(1, 2, " items", &path, "pair", &mut issues);
    rules::min_num(0.0, 1.0, "1", &path, "quantity", &mut issues);
    rules::max_num(200.0, 150.0, "150", &path, "age", &mut issues);
    rules::one_of("void", &["draft", "sent"], &path, "status", &mut issues);
    rules::email("nope", &path, "email", &mut issues);
    rules::url("nope", &path, "site", &mut issues);
    rules::uuid("nope", &path, "token", &mut issues);
    let rendered: Vec<String> = issues.iter().map(|i| i.to_string()).collect();
    assert_eq!(
        rendered,
        vec![
            "lines.0.description: is required",
            "lines.0.description: must be at least 3 characters",
            "lines.0.tags: must be at most 2 items",
            "lines.0.pair: must be exactly 2 items",
            "lines.0.quantity: must be at least 1",
            "lines.0.age: must be at most 150",
            "lines.0.status: must be one of draft sent",
            "lines.0.email: must be a valid email address",
            "lines.0.site: must be a valid URL",
            "lines.0.token: must be a valid UUID",
        ]
    );
    assert!(rules::is_email("ada@example.com") && !rules::is_email("ada@"));
    assert!(rules::is_url("https://example.com/x") && !rules::is_url("example.com"));
    assert!(rules::is_uuid("123e4567-e89b-12d3-a456-426614174000"));
}
