module github.com/bowlinedev/bowline/examples/routers/fiber

go 1.25.0

require (
	github.com/bowlinedev/bowline v1.4.1
	github.com/bowlinedev/bowline/adapters/fiber v0.0.0
	github.com/gofiber/fiber/v2 v2.52.15
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/molecule-man/go-brrr v1.0.1 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.74.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace github.com/bowlinedev/bowline => ../../..

replace github.com/bowlinedev/bowline/adapters/fiber => ../../../adapters/fiber
