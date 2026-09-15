module github.com/bowlinedev/bowline/examples/go-client

go 1.24

require (
	github.com/bowlinedev/bowline v0.3.1-0.20260915204325-ba2c558170f5
	github.com/bowlinedev/bowline/examples/ledger v0.0.0
)

replace github.com/bowlinedev/bowline => ../..

replace github.com/bowlinedev/bowline/examples/ledger => ../ledger

replace github.com/bowlinedev/bowline/transport/websocket => ../../transport/websocket

replace github.com/bowlinedev/bowline/mcp => ../../mcp

replace github.com/bowlinedev/bowline/playground => ../../playground

replace github.com/bowlinedev/bowline/contracttest => ../../contracttest
