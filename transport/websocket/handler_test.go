package websocket

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bowlinedev/bowline"
	"github.com/coder/websocket"
)

type countInput struct {
	Count int64 `json:"count" validate:"min=1"`
}

type tick struct {
	N    int64    `json:"n"`
	Tags []string `json:"tags"`
}

func counter(ctx context.Context, in countInput, stream *bowline.Stream[tick]) error {
	for i := int64(1); i <= in.Count; i++ {
		if err := stream.Send(tick{N: i}); err != nil {
			return err
		}
	}
	if in.Count == 2 {
		return bowline.Errorf(bowline.NotFound, "gone")
	}
	return nil
}

func forever(ctx context.Context, in struct{}, stream *bowline.Stream[tick]) error {
	for {
		if err := stream.Send(tick{}); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func dial(t *testing.T) (*websocket.Conn, func()) {
	t.Helper()
	router := bowline.NewRouter(bowline.Subscription("count", counter), bowline.Subscription("forever", forever))
	server := httptest.NewServer(Handler(router, Options{}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	return conn, func() {
		conn.Close(websocket.StatusNormalClosure, "")
		cancel()
		server.Close()
	}
}

func send(t *testing.T, conn *websocket.Conn, f frame) {
	t.Helper()
	data, _ := json.Marshal(f)
	if err := conn.Write(context.Background(), websocket.MessageText, data); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, conn *websocket.Conn) frame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var f frame
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestMultiplexedSubscriptions(t *testing.T) {
	conn, done := dial(t)
	defer done()
	send(t, conn, frame{ID: 1, Type: "subscribe", Path: "count", Input: json.RawMessage(`{"count":3}`)})
	send(t, conn, frame{ID: 2, Type: "subscribe", Path: "count", Input: json.RawMessage(`{"count":2}`)})
	got := map[int64][]string{}
	for len(got[1]) < 4 || len(got[2]) < 3 {
		f := read(t, conn)
		if f.Type == "data" {
			got[f.ID] = append(got[f.ID], string(f.Data))
		} else {
			got[f.ID] = append(got[f.ID], f.Type+":"+string(f.Error))
		}
	}
	if got[1][3] != "done:" || !strings.Contains(got[1][0], `"tags":[]`) {
		t.Fatalf("stream 1: %v", got[1])
	}
	if !strings.Contains(got[2][2], `"NOT_FOUND"`) {
		t.Fatalf("stream 2: %v", got[2])
	}
}

func TestStopCancelsASubscription(t *testing.T) {
	conn, done := dial(t)
	defer done()
	send(t, conn, frame{ID: 7, Type: "subscribe", Path: "forever"})
	read(t, conn)
	send(t, conn, frame{ID: 7, Type: "stop"})
	send(t, conn, frame{ID: 8, Type: "subscribe", Path: "count", Input: json.RawMessage(`{"count":1}`)})
	sawDone := false
	for i := 0; i < 50 && !sawDone; i++ {
		f := read(t, conn)
		if f.ID == 8 && f.Type == "done" {
			sawDone = true
		}
	}
	if !sawDone {
		t.Fatal("stream 8 never completed after stopping stream 7")
	}
}

func TestUnknownPathFrame(t *testing.T) {
	conn, done := dial(t)
	defer done()
	send(t, conn, frame{ID: 3, Type: "subscribe", Path: "nope"})
	f := read(t, conn)
	if f.Type != "error" || !strings.Contains(string(f.Error), "UNIMPLEMENTED") {
		t.Fatalf("%+v", f)
	}
	send(t, conn, frame{ID: 3, Type: "subscribe", Path: "count", Input: json.RawMessage(`{"count":0}`)})
	f = read(t, conn)
	if f.Type != "error" || !strings.Contains(string(f.Error), "INVALID_ARGUMENT") {
		t.Fatalf("%+v", f)
	}
}
