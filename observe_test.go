package bowline_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline"
)

type obsIn struct {
	Name string `json:"name" validate:"required"`
}

type obsOut struct {
	Greeting string `json:"greeting"`
}

type recorder struct {
	ready    []string
	started  int
	finished []error
}

func (r *recorder) HandlerReady(procs []*bowline.Procedure) {
	for _, p := range procs {
		r.ready = append(r.ready, p.Path)
	}
}

func (r *recorder) CallStarted(ctx context.Context, call *bowline.Call) context.Context {
	r.started++
	return context.WithValue(ctx, ctxKey{}, "traced")
}

func (r *recorder) CallFinished(ctx context.Context, call *bowline.Call, err error) {
	r.finished = append(r.finished, err)
}

type ctxKey struct{}

func obsRouter(seen *string) *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("greet", func(ctx context.Context, in obsIn) (obsOut, error) {
			if v, ok := ctx.Value(ctxKey{}).(string); ok {
				*seen = v
			}
			if in.Name == "boom" {
				return obsOut{}, errors.New("boom")
			}
			return obsOut{Greeting: "hello, " + in.Name}, nil
		}),
	)
}

func serveObs(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/greet", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestObserverSeesTheLifecycle(t *testing.T) {
	rec := &recorder{}
	var seen string
	h := obsRouter(&seen).Handler(bowline.Observe(rec))
	if len(rec.ready) != 1 || rec.ready[0] != "greet" {
		t.Fatalf("HandlerReady got %v", rec.ready)
	}
	serveObs(t, h, `{"name":"ada"}`)
	if rec.started != 1 || len(rec.finished) != 1 {
		t.Fatalf("started %d finished %d", rec.started, len(rec.finished))
	}
	if rec.finished[0] != nil {
		t.Fatalf("a successful call reported %v", rec.finished[0])
	}
	if seen != "traced" {
		t.Fatal("the context returned by CallStarted did not reach the procedure")
	}
}

func TestObserverSeesTheError(t *testing.T) {
	rec := &recorder{}
	var seen string
	h := obsRouter(&seen).Handler(bowline.Observe(rec), bowline.Production(true))
	serveObs(t, h, `{"name":"boom"}`)
	if len(rec.finished) != 1 || rec.finished[0] == nil {
		t.Fatalf("a failing call reported %v", rec.finished)
	}
}

func TestTypedMiddlewareSeesTheTypedInput(t *testing.T) {
	var got string
	mw := bowline.Typed(func(ctx context.Context, in obsIn, next bowline.TypedNext[obsIn, obsOut]) (obsOut, error) {
		got = in.Name
		out, err := next(ctx, obsIn{Name: strings.ToUpper(in.Name)})
		if err != nil {
			return out, err
		}
		out.Greeting += "!"
		return out, nil
	})
	var seen string
	h := obsRouter(&seen).Use(mw).Handler()
	w := serveObs(t, h, `{"name":"ada"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	if got != "ada" {
		t.Fatalf("middleware saw %q", got)
	}
	if !strings.Contains(w.Body.String(), "hello, ADA!") {
		t.Fatalf("middleware did not transform input and output: %s", w.Body)
	}
}

func TestNoObserverCostsNothing(t *testing.T) {
	var seen string
	h := obsRouter(&seen).Handler()
	if w := serveObs(t, h, `{"name":"ada"}`); w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}
