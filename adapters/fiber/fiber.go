package bowlinefiber

import (
	"net/http"

	"github.com/bowlinedev/bowline"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

func Handler(r *bowline.Router, opts ...bowline.HandlerOption) fiber.Handler {
	h := fasthttpadaptor.NewFastHTTPHandler(r.Handler(opts...))
	return func(c *fiber.Ctx) error {
		h(c.Context())
		return nil
	}
}

func Mount(app fiber.Router, prefix string, r *bowline.Router, opts ...bowline.HandlerOption) {
	app.All(prefix+"/*", Handler(r, opts...))
}

func HTTPHandler(app *fiber.App) http.Handler {
	return adaptor.FiberApp(app)
}
