package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/ledger/api"
	bowlinews "github.com/bowlinedev/bowline/transport/websocket"
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
	production := os.Getenv("ENV") == "production"
	options := []bowline.HandlerOption{
		bowline.Production(production),
		bowline.Idempotency(bowline.MemoryIdempotencyStore(), 24*time.Hour),
		bowline.Heartbeat(15 * time.Second),
		bowline.MaxUploadSize(8 << 20),
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Mount("/api", routes.Handler(options...))
	r.Handle("/ws", bowlinews.Handler(routes, bowlinews.Options{OriginPatterns: []string{"localhost:*", "127.0.0.1:*"}, Handler: options}))
	slog.Info("ledger listening", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
