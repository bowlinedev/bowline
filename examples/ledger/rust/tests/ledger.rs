#[path = "../src/bowline.rs"]
#[allow(dead_code)]
#[rustfmt::skip]
mod bowline;

use bowline::{
    CreateInvoiceInput, GetInvoiceInput, Line, ListInvoicesInput, Status, VoidInvoiceInput,
    WatchInput,
};
use bowline_client::{Code, Error, Transport};
use futures_util::StreamExt;
use std::net::TcpListener;
use std::process::{Child, Command, Stdio};
use std::time::{Duration, Instant};

struct Server {
    child: Child,
    url: String,
}

impl Drop for Server {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

fn free_port() -> u16 {
    TcpListener::bind("127.0.0.1:0")
        .unwrap()
        .local_addr()
        .unwrap()
        .port()
}

async fn start() -> Server {
    let port = free_port();
    let child = Command::new("go")
        .args(["run", "./cmd/server"])
        .current_dir(concat!(env!("CARGO_MANIFEST_DIR"), "/.."))
        .env("ADDR", format!("127.0.0.1:{port}"))
        .env("LEDGER_FIXED_TIME", "2026-09-15T12:00:00Z")
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .expect("go run ./cmd/server");
    let url = format!("http://127.0.0.1:{port}/api");
    let server = Server { child, url };
    let http = reqwest::Client::new();
    let deadline = Instant::now() + Duration::from_secs(120);
    loop {
        if let Ok(resp) = http.get(format!("{}/health", server.url)).send().await {
            if resp.status().is_success() {
                return server;
            }
        }
        if Instant::now() > deadline {
            panic!("ledger server did not start");
        }
        tokio::time::sleep(Duration::from_millis(200)).await;
    }
}

fn line(description: &str, quantity: i32) -> CreateInvoiceInput {
    CreateInvoiceInput {
        customer_id: 1,
        lines: vec![Line {
            description: description.to_string(),
            quantity,
            unit_price: "USD 10.00".to_string(),
        }],
        note: None,
    }
}

#[tokio::test]
async fn list_create_void_and_watch_against_the_ledger() {
    let server = start().await;
    let client = bowline::Client::new(Transport::new(server.url.clone()));
    let invoices = client.invoices();

    let page = invoices
        .list(
            &ListInvoicesInput {
                cursor: None,
                limit: 20,
                status: None,
            },
            None,
        )
        .await
        .unwrap();
    let ids: Vec<i64> = page.items.iter().map(|i| i.id).collect();
    assert_eq!(ids, vec![3, 4]);
    assert_eq!(page.items[0].total, "USD 1500.00");
    assert_eq!(page.items[0].status, Status::Sent);
    assert_eq!(
        page.items[0].created_at.to_rfc3339(),
        "2026-09-15T12:00:00+00:00"
    );

    let local = invoices.create(&line("", 0), None).await.unwrap_err();
    match local {
        Error::Invalid(issues) => {
            let paths: Vec<String> = issues.iter().map(|i| i.path.join(".")).collect();
            assert_eq!(paths, vec!["lines.0.description", "lines.0.quantity"]);
        }
        other => panic!("expected local validation, got {other:?}"),
    }

    let remote = client
        .transport()
        .call::<_, bowline::Invoice>(
            "invoices.create",
            bowline_client::Method::Post,
            &Unchecked(line("", 0)),
            None,
        )
        .await
        .unwrap_err();
    let remote = remote.remote().expect("remote error");
    assert_eq!(remote.code, Code::InvalidArgument);
    let paths: Vec<String> = remote.issues.iter().map(|i| i.path.join(".")).collect();
    assert_eq!(paths, vec!["lines.0.description", "lines.0.quantity"]);
    assert_eq!(remote.issues[0].message, "is required");
    assert_eq!(remote.issues[1].message, "must be at least 1");

    let mut watch = Box::pin(invoices.watch(&WatchInput { status: None }, None));
    let first = tokio::spawn(async move { watch.next().await });
    tokio::time::sleep(Duration::from_millis(500)).await;

    let created = invoices.create(&line("Widgets", 3), None).await.unwrap();
    assert_eq!(created.total, "USD 30.00");
    assert_eq!(created.status, Status::Draft);
    let fetched = invoices
        .get(&GetInvoiceInput { id: created.id }, None)
        .await
        .unwrap();
    assert_eq!(fetched.lines[0].description, "Widgets");

    let event = tokio::time::timeout(Duration::from_secs(10), first)
        .await
        .expect("watch event")
        .expect("watch task")
        .expect("stream open")
        .expect("event ok");
    assert_eq!(event.id, created.id);

    let locked = invoices
        .void(&VoidInvoiceInput { id: 4 }, None)
        .await
        .unwrap_err();
    assert_eq!(locked.code(), Some(Code::FailedPrecondition));
    assert_eq!(locked.variant(), Some("InvoiceLocked"));
    let details: bowline::InvoiceLocked = locked.details_as().unwrap();
    assert_eq!((details.id, details.status), (4, Status::Paid));

    let missing = invoices
        .get(&GetInvoiceInput { id: 999 }, None)
        .await
        .unwrap_err();
    assert_eq!(missing.code(), Some(Code::NotFound));

    let voided = invoices
        .void(&VoidInvoiceInput { id: created.id }, None)
        .await
        .unwrap();
    assert_eq!(voided.status, Status::Void);
}

struct Unchecked(CreateInvoiceInput);

impl serde::Serialize for Unchecked {
    fn serialize<S: serde::Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        self.0.serialize(serializer)
    }
}

impl bowline_client::Validate for Unchecked {
    fn validate(&self, _path: &mut Vec<String>, _issues: &mut Vec<bowline_client::Issue>) {}
}
