package bowlinefiber

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/conformance"
	"github.com/gofiber/fiber/v2"
)

func serve(t *testing.T, app *fiber.App) string {
	t.Helper()
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
			return "http://" + ln.Addr().String()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("fiber did not start listening")
	return ""
}

func TestConformance(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	Mount(app, "/api", conformance.Router(), conformance.Options()...)
	base := serve(t, app)
	conformance.RunURL(t, base+"/api")
}

func TestMountUnderPrefix(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	Mount(app.Group("/v1"), "", conformance.Router(), conformance.Options()...)
	base := serve(t, app)
	resp, err := http.Get(base + "/v1/nested.deep.get")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != `{"depth":2}` {
		t.Fatalf("nested: %d %s", resp.StatusCode, body)
	}
	req, _ := http.NewRequest(http.MethodDelete, base+"/v1/create", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 405 || resp.Header.Get("Allow") != "POST" {
		t.Fatalf("delete: %d allow %q", resp.StatusCode, resp.Header.Get("Allow"))
	}
}

func TestHTTPHandler(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	Mount(app, "/api", conformance.Router(), conformance.Options()...)
	conformance.Run(t, func(http.Handler) http.Handler { return HTTPHandler(app) })
}
