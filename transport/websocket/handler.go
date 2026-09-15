package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/bowlinedev/bowline"
	"github.com/coder/websocket"
)

type Options struct {
	OriginPatterns []string
	Handler        []bowline.HandlerOption
}

func Handler(r *bowline.Router, opts Options) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := websocket.Accept(w, req, &websocket.AcceptOptions{OriginPatterns: opts.OriginPatterns})
		if err != nil {
			return
		}
		s := &session{router: r, conn: conn, opts: opts.Handler, cancels: map[int64]context.CancelFunc{}}
		s.run(req.Context())
	})
}

type session struct {
	router  *bowline.Router
	conn    *websocket.Conn
	opts    []bowline.HandlerOption
	writeMu sync.Mutex
	mu      sync.Mutex
	cancels map[int64]context.CancelFunc
	wg      sync.WaitGroup
}

func (s *session) run(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	defer s.wg.Wait()
	defer s.conn.Close(websocket.StatusNormalClosure, "")
	for {
		_, data, err := s.conn.Read(ctx)
		if err != nil {
			s.stopAll()
			return
		}
		var f frame
		if err := json.Unmarshal(data, &f); err != nil {
			s.write(errorFrame(f.ID, []byte(fmt.Sprintf(`{"error":{"code":"INVALID_ARGUMENT","message":%q}}`, "malformed frame: "+err.Error()))))
			continue
		}
		switch f.Type {
		case "subscribe":
			s.subscribe(ctx, f)
		case "stop":
			s.stop(f.ID)
		default:
			s.write(errorFrame(f.ID, []byte(fmt.Sprintf(`{"error":{"code":"INVALID_ARGUMENT","message":%q}}`, "unknown frame type "+f.Type))))
		}
	}
}

func (s *session) subscribe(ctx context.Context, f frame) {
	s.mu.Lock()
	if _, exists := s.cancels[f.ID]; exists {
		s.mu.Unlock()
		s.write(errorFrame(f.ID, []byte(`{"error":{"code":"ALREADY_EXISTS","message":"subscription id is in use"}}`)))
		return
	}
	subCtx, cancel := context.WithCancel(ctx)
	s.cancels[f.ID] = cancel
	s.mu.Unlock()
	input := f.Input
	if len(input) == 0 {
		input = []byte("{}")
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.stop(f.ID)
		err := s.router.Subscribe(subCtx, f.Path, input, func(data []byte) error {
			return s.write(frame{ID: f.ID, Type: "data", Data: data})
		}, s.opts...)
		if subCtx.Err() != nil {
			return
		}
		var failure *bowline.StreamFailure
		switch {
		case err == nil:
			s.write(frame{ID: f.ID, Type: "done"})
		case errors.As(err, &failure):
			s.write(errorFrame(f.ID, failure.Body))
		default:
			s.write(errorFrame(f.ID, []byte(`{"error":{"code":"INTERNAL","message":"internal error"}}`)))
		}
	}()
}

func (s *session) stop(id int64) {
	s.mu.Lock()
	cancel, ok := s.cancels[id]
	delete(s.cancels, id)
	s.mu.Unlock()
	if ok {
		cancel()
	}
}

func (s *session) stopAll() {
	s.mu.Lock()
	cancels := s.cancels
	s.cancels = map[int64]context.CancelFunc{}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (s *session) write(f frame) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.Write(context.Background(), websocket.MessageText, data)
}
