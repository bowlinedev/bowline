# Rust

The `rust` target generates a single file, `bowline.rs`, with serde structs, `impl Validate` blocks, and sub-clients that borrow the transport. The runtime is the `bowline-client` crate.

```json
{ "targets": { "rust": { "out": "rust/src/bowline.rs" } } }
```

```toml
[dependencies]
bowline-client = "1.0"
serde = { version = "1", features = ["derive"] }
serde_json = "1"
chrono = { version = "0.4", features = ["serde"] }
```

## Calling

source: examples/ledger/rust/src/main.rs:14-18

```rust
fn client() -> Client {
    let url =
        std::env::var("LEDGER_URL").unwrap_or_else(|_| "http://localhost:8080/api".to_string());
    Client::new(Transport::new(url))
}
```

source: examples/ledger/rust/src/main.rs:61-72

```rust
async fn list(client: &Client) -> Result<(), Error> {
    let page = client
        .invoices()
        .list(
            &ListInvoicesInput {
                cursor: None,
                limit: 20,
                status: None,
            },
            None,
        )
        .await?;
```

Each mount is an accessor that returns a client borrowing the transport. Every procedure is an async method that takes the input by reference and an `Option<&CallOptions>`. Subscriptions return a `Stream` of results. Uploads take a `reqwest::Body` and a file name. `Transport::call` runs `Validate` first and returns `Error::Invalid` without making a request if the input breaks a rule.

## Errors

source: examples/ledger/rust/src/main.rs:20-37

```rust
fn report(err: Error) -> ExitCode {
    match err {
        Error::Remote(remote) if remote.code == Code::InvalidArgument => {
            eprintln!("rejected: {}", remote.message);
            for issue in remote.issues {
                eprintln!("  {}: {}", issue.path.join("."), issue.message);
            }
        }
        Error::Invalid(issues) => {
            eprintln!("rejected before sending:");
            for issue in issues {
                eprintln!("  {}: {}", issue.path.join("."), issue.message);
            }
        }
        other => eprintln!("error: {other}"),
    }
    ExitCode::FAILURE
}
```

`Error::Remote` holds the envelope, with `code`, `message`, `status`, the declared variant name, raw details with a `details_as` helper, and `issues`. `Error::Transport` wraps reqwest failures. `Error::Decode` wraps serde failures.

## Types

Fields are `snake_case` with `#[serde(rename)]` for the JSON name. Integers keep their widths. `,string` integers use the `codec::string_int` serde module. Timestamps are `chrono::DateTime<Utc>`. Maps are `BTreeMap` so that output is deterministic. Fixed-length arrays are real arrays. A self-referential type is boxed where the cycle would otherwise be infinite-sized. The full table is the Rust column of `spec/mapping-table.md`. The generated file has `#[rustfmt::skip]` so that `cargo fmt --check` and `bowline check` agree.

To verify: `cd examples/ledger/rust && cargo test` starts the Go server and runs list, create, both validation paths, void, and a subscription. `scripts/check-goldens.sh rust` compiles a golden for every fidelity row under `clippy -D warnings`.
