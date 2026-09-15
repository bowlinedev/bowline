module github.com/bowlinedev/bowline/examples/ledger

go 1.24

require (
	github.com/bowlinedev/bowline v0.2.1-0.20260915195321-b994a91e81b1
	github.com/bowlinedev/bowline/mcp v0.0.0
	github.com/bowlinedev/bowline/transport/websocket v0.0.0
	github.com/go-chi/chi/v5 v5.2.2
)

require github.com/coder/websocket v1.8.15 // indirect

replace github.com/bowlinedev/bowline => ../..

replace github.com/bowlinedev/bowline/transport/websocket => ../../transport/websocket

replace github.com/bowlinedev/bowline/mcp => ../../mcp
