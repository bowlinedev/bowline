module github.com/bowlinedev/bowline/examples/routers/connect

go 1.25.0

require (
	connectrpc.com/connect v1.21.0
	github.com/bowlinedev/bowline v0.0.0
)

require google.golang.org/protobuf v1.36.11 // indirect

replace github.com/bowlinedev/bowline => ../../..
