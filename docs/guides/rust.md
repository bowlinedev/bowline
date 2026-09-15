# Rust

The `rust` target generates one file, `bowline.rs`, with serde structs, `impl Validate` blocks, and borrowed sub-clients. The runtime is the `bowline-client` crate.

```json
{ "targets": { "rust": { "out": "rust/src/bowline.rs" } } }
```

```toml
[dependencies]
bowline-client = "0.5"
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

Each mount is an accessor returning a client that borrows the transport; every procedure is an async method taking the input by reference and `Option<&CallOptions>`. Subscriptions return a `Stream` of results and uploads take a `reqwest::Body` and a file name. `Transport::call` runs `Validate` first and returns `Error::Invalid` without a request when the input breaks a rule.

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

`Error::Remote` holds the envelope with `code`, `message`, `status`, the declared variant name, raw details with `details_as`, and `issues`; `Error::Transport` wraps reqwest failures; `Error::Decode` wraps serde failures.

## Types

Fields are `snake_case` with `#[serde(rename)]` carrying the JSON name; integers keep their widths; `,string` integers use the `codec::string_int` serde module; timestamps are `chrono::DateTime<Utc>`; maps are `BTreeMap` so output is deterministic; fixed-length arrays are real arrays; a self-referential type is boxed where the cycle would be infinite-sized. The full table is the Rust column of `spec/mapping-table.md`. The generated file is declared with `#[rustfmt::skip]` so `cargo fmt --check` and `bowline check` agree.

Proof: `cd examples/ledger/rust && cargo test` starts the Go server and drives list, create, both validation paths, void, and a subscription; `scripts/check-goldens.sh rust` compiles a golden for every fidelity row under `clippy -D warnings`.
