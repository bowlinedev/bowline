package main

import (
	"log"
	"net/http"
	"os"

	"github.com/bowlinedev/bowline/conformance"
)

func Mount(h http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/", h)
	return mux
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Fatal(http.ListenAndServe(addr, Mount(conformance.Router().Handler(conformance.Options()...))))
}
