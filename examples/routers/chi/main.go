package main

import (
	"log"
	"net/http"
	"os"

	"github.com/bowlinedev/bowline/conformance"
	"github.com/go-chi/chi/v5"
)

func Mount(h http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Mount("/api", h)
	return r
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Fatal(http.ListenAndServe(addr, Mount(conformance.Router().Handler(conformance.Options()...))))
}
