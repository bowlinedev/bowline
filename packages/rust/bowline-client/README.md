# bowline-client

The Rust client runtime for [Bowline](https://github.com/bowlinedev/bowline).

Bowline turns your Go code into typed API clients. This crate is what the generated `bowline.rs` uses to make calls.

## Install

```bash
cargo add bowline-client
```

## Use

```rust
use bowline_client::Transport;

let client = bowline::Client::new(Transport::new("http://localhost:8080/api"));
let invoice = client.invoices().get(GetInput { id: 3 }).await?;
```

You do not write the types. `bowline gen` writes them from your Go code.

Errors come back as `Error` with the same codes the Go server uses. Queries are sent as `GET`, mutations as `POST`. Inputs are checked before the request leaves.

Docs: [bowlinedev/bowline](https://github.com/bowlinedev/bowline)

Apache-2.0
