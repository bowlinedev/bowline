package main

import (
	"log"
	"os"

	bowlinefiber "github.com/bowlinedev/bowline/adapters/fiber"
	"github.com/bowlinedev/bowline/conformance"
	"github.com/gofiber/fiber/v2"
)

func App() *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	bowlinefiber.Mount(app, "/api", conformance.Router(), conformance.Options()...)
	return app
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("fiber example listening on %s", addr)
	log.Fatal(App().Listen(addr))
}
