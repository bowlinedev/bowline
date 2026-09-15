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
	routes := api.Routes()
	if os.Getenv("ENV") != "production" {
		if err := routes.Verify(api.Contract); err != nil {
			slog.Error("contract drift", "error", err)
			os.Exit(1)
		}
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Mount("/api", routes.Handler(bowline.Production(os.Getenv("ENV") == "production")))
	slog.Info("ledger listening", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
