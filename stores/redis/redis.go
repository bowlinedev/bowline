package redis

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/bowlinedev/bowline"
	"github.com/redis/go-redis/v9"
)

const DefaultPrefix = "bowline:idem:"

type Option func(*Store)

func WithPrefix(prefix string) Option {
	return func(s *Store) { s.prefix = prefix }
}

func WithClaimTTL(d time.Duration) Option {
	return func(s *Store) { s.claimTTL = d }
}

type Store struct {
	client   redis.Cmdable
	prefix   string
	claimTTL time.Duration
}

func New(client redis.Cmdable, opts ...Option) (*Store, error) {
	if client == nil {
		return nil, errors.New("bowline redis store: nil client")
	}
	s := &Store{client: client, prefix: DefaultPrefix, claimTTL: time.Minute}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

func (s *Store) name(key string) string {
	return s.prefix + key
}

const (
	markerInFlight byte = 0
	markerStored   byte = 1
)

func encode(status int, body []byte) []byte {
	out := make([]byte, 5, 5+len(body))
	out[0] = markerStored
	binary.BigEndian.PutUint32(out[1:5], uint32(status))
	return append(out, body...)
}

func decode(raw []byte) (bowline.IdempotencyState, int, []byte, error) {
	if len(raw) == 0 {
		return bowline.IdempotencyNew, 0, nil, errors.New("bowline redis store: empty record")
	}
	switch raw[0] {
	case markerInFlight:
		return bowline.IdempotencyInFlight, 0, nil, nil
	case markerStored:
		if len(raw) < 5 {
			return bowline.IdempotencyNew, 0, nil, errors.New("bowline redis store: truncated record")
		}
		status := int(binary.BigEndian.Uint32(raw[1:5]))
		body := make([]byte, len(raw)-5)
		copy(body, raw[5:])
		return bowline.IdempotencyStored, status, body, nil
	}
	return bowline.IdempotencyNew, 0, nil, fmt.Errorf("bowline redis store: unknown record marker %d", raw[0])
}

func (s *Store) Begin(ctx context.Context, key string) (bowline.IdempotencyState, int, []byte, error) {
	name := s.name(key)
	claimed, err := s.client.SetNX(ctx, name, []byte{markerInFlight}, s.claimTTL).Result()
	if err != nil {
		return bowline.IdempotencyNew, 0, nil, fmt.Errorf("bowline redis store: claiming %s: %w", key, err)
	}
	if claimed {
		return bowline.IdempotencyNew, 0, nil, nil
	}
	held, err := s.client.Get(ctx, name).Bytes()
	switch {
	case errors.Is(err, redis.Nil):
		return bowline.IdempotencyNew, 0, nil, nil
	case err != nil:
		return bowline.IdempotencyNew, 0, nil, fmt.Errorf("bowline redis store: reading %s: %w", key, err)
	}
	return decode(held)
}

func (s *Store) Complete(ctx context.Context, key string, status int, body []byte, ttl time.Duration) error {
	if err := s.client.Set(ctx, s.name(key), encode(status, body), ttl).Err(); err != nil {
		return fmt.Errorf("bowline redis store: storing %s: %w", key, err)
	}
	return nil
}

func (s *Store) Abort(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, s.name(key)).Err(); err != nil {
		return fmt.Errorf("bowline redis store: releasing %s: %w", key, err)
	}
	return nil
}
