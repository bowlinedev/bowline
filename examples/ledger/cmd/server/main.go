package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/ledger/api"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Mount("/api", api.Routes().Handler(bowline.Production(os.Getenv("ENV") == "production")))
	slog.Info("ledger listening", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
