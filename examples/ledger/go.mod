module github.com/bowlinedev/bowline/examples/ledger

go 1.24

require (
	github.com/bowlinedev/bowline v0.3.1-0.20260915204325-ba2c558170f5
	github.com/bowlinedev/bowline/contracttest v0.0.0
	github.com/bowlinedev/bowline/mcp v0.0.0
	github.com/bowlinedev/bowline/playground v0.0.0
	github.com/bowlinedev/bowline/transport/websocket v0.0.0
	github.com/go-chi/chi/v5 v5.2.2
)

require github.com/coder/websocket v1.8.15 // indirect

replace github.com/bowlinedev/bowline => ../..

replace github.com/bowlinedev/bowline/transport/websocket => ../../transport/websocket

replace github.com/bowlinedev/bowline/mcp => ../../mcp

replace github.com/bowlinedev/bowline/playground => ../../playground

replace github.com/bowlinedev/bowline/contracttest => ../../contracttest
