module github.com/bowlinedev/bowline/examples/routers/fiber

go 1.25.0

require (
	github.com/bowlinedev/bowline v0.0.0
	github.com/bowlinedev/bowline/adapters/fiber v0.0.0
	github.com/gofiber/fiber/v2 v2.52.15
)

replace github.com/bowlinedev/bowline => ../../..

replace github.com/bowlinedev/bowline/adapters/fiber => ../../../adapters/fiber
