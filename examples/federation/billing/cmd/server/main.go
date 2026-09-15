package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/federation/billing/api"
	"github.com/bowlinedev/bowline/signing"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8081"
	}
	routes := api.Routes()
	if err := routes.Verify(api.Contract); err != nil {
		slog.Error("contract drift", "error", err)
		os.Exit(1)
	}
	options := []bowline.HandlerOption{
		bowline.WithContract(api.Contract),
		bowline.Idempotency(bowline.MemoryIdempotencyStore(), 0),
	}
	if secret := os.Getenv("BILLING_INBOUND_SECRET"); secret != "" {
		options = append(options, bowline.Signed(signing.StaticSecrets{os.Getenv("BILLING_INBOUND_KEY"): []byte(secret)}))
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", routes.Handler(options...)))
	slog.Info("billing listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
