package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/bowlinedev/bowline/conformance"
)

type PingRequest struct {
	Message string `json:"message"`
}

type PingResponse struct {
	Message string `json:"message"`
}

const PingProcedure = "/ping.PingService/Ping"

func ping(ctx context.Context, req *connect.Request[PingRequest]) (*connect.Response[PingResponse], error) {
	return connect.NewResponse(&PingResponse{Message: "pong: " + req.Msg.Message}), nil
}

func Mount(h http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/", h)
	mux.Handle(PingProcedure, connect.NewUnaryHandler(PingProcedure, ping, connect.WithCodec(jsonCodec{})))
	return mux
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Fatal(http.ListenAndServe(addr, Mount(conformance.Router().Handler(conformance.Options()...))))
}
