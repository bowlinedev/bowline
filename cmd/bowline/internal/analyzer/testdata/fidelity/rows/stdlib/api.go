package stdlib

import (
	"context"
	"encoding/json"
	"net/netip"
	"time"

	"github.com/bowlinedev/bowline"
)

type Event struct {
	At       time.Time       `json:"at"`
	Took     time.Duration   `json:"took"`
	Payload  json.RawMessage `json:"payload"`
	Addr     netip.Addr      `json:"addr"`
	Big      int64           `json:"big,string"`
	Unsigned uint64          `json:"unsigned,string"`
}

func Get(ctx context.Context, in struct{}) (Event, error) { return Event{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
