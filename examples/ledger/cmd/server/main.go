package main

import (
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/ledger/api"
	"github.com/bowlinedev/bowline/mcp"
	"github.com/bowlinedev/bowline/playground"
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
	handler, err := newHandler(routes, os.Getenv("ENV") == "production")
	if err != nil {
		slog.Error("building handler", "error", err)
		os.Exit(1)
	}
	slog.Info("ledger listening", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func browserOptions() []bowline.HandlerOption {
	if os.Getenv("CSRF") != "on" {
		return nil
	}
	return []bowline.HandlerOption{bowline.CSRF(bowline.CSRFOptions{
		TrustFetchMetadata: true,
		AllowedOrigins:     allowedOrigins(),
	})}
}

func allowedOrigins() []string {
	var origins []string
	for _, origin := range strings.Split(os.Getenv("CSRF_ORIGINS"), ",") {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

func newHandler(routes *bowline.Router, production bool) (http.Handler, error) {
	options := []bowline.HandlerOption{
		bowline.Production(production),
		bowline.WithContract(api.Contract),
		bowline.Idempotency(bowline.MemoryIdempotencyStore(), 24*time.Hour),
		bowline.Heartbeat(15 * time.Second),
		bowline.MaxUploadSize(8 << 20),
		bowline.SecurityHeaders(),
	}
	tools, err := mcp.Handler(routes, api.Contract, mcp.Runtime(options...), mcp.RateLimit(120, 20))
	if err != nil {
		return nil, err
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Mount("/api", routes.Handler(slices.Concat(options, browserOptions())...))
	r.Handle("/ws", bowlinews.Handler(routes, bowlinews.Options{OriginPatterns: []string{"localhost:*", "127.0.0.1:*"}, Handler: options}))
	r.Handle("/mcp", tools)
	if !production {
		r.Handle("/playground/*", http.StripPrefix("/playground", playground.New(api.Contract, playground.WithUpstream("/api"), playground.WithTitle("Ledger playground"))))
	}
	return r, nil
}
