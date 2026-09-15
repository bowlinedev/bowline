package main

import (
	"net"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/conformance"
)

func TestConformance(t *testing.T) {
	app := App()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", ln.Addr().String(), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	conformance.RunURL(t, "http://"+ln.Addr().String()+"/api")
}
