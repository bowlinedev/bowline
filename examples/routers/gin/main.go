package main

import (
	"log"
	"net/http"
	"os"

	"github.com/bowlinedev/bowline/conformance"
	"github.com/gin-gonic/gin"
)

func Mount(h http.Handler) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false
	r.RemoveExtraSlash = false
	r.Any("/api/*path", gin.WrapH(h))
	return r
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Fatal(http.ListenAndServe(addr, Mount(conformance.Router().Handler(conformance.Options()...))))
}
