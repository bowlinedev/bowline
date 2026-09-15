package main

import (
	"log"
	"net/http"
	"os"

	"github.com/bowlinedev/bowline/conformance"
	"github.com/labstack/echo/v4"
)

func Mount(h http.Handler) http.Handler {
	e := echo.New()
	e.HideBanner = true
	e.Any("/api/*", echo.WrapHandler(h))
	return e
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Fatal(http.ListenAndServe(addr, Mount(conformance.Router().Handler(conformance.Options()...))))
}
