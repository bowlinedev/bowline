package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("registry: not found")

type Service struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Owners      []string  `json:"owners,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Version struct {
	Service     string          `json:"service"`
	Hash        string          `json:"hash"`
	PublishedAt time.Time       `json:"publishedAt"`
	Ref         string          `json:"ref,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Contract    json.RawMessage `json:"contract"`
}

type Consumer struct {
	Consumer     string          `json:"consumer"`
	Provider     string          `json:"provider"`
	ProviderHash string          `json:"providerHash,omitempty"`
	RecordedAt   time.Time       `json:"recordedAt"`
	Usage        json.RawMessage `json:"usage"`
}

type Composition struct {
	Gateway     string            `json:"gateway"`
	Hash        string            `json:"hash"`
	Services    map[string]string `json:"services"`
	PublishedAt time.Time         `json:"publishedAt"`
}

type Store interface {
	PutService(ctx context.Context, s Service) error
	Services(ctx context.Context) ([]Service, error)
	Service(ctx context.Context, name string) (Service, error)
	PutVersion(ctx context.Context, v Version) (created bool, err error)
	Versions(ctx context.Context, service string) ([]Version, error)
	Version(ctx context.Context, service, hash string) (Version, error)
	Tag(ctx context.Context, service, tag, hash string) error
	Tagged(ctx context.Context, service, tag string) (Version, error)
	PutConsumer(ctx context.Context, c Consumer) error
	Consumers(ctx context.Context, provider string) ([]Consumer, error)
	PutComposition(ctx context.Context, c Composition) error
	Compositions(ctx context.Context, service string) ([]Composition, error)
}

func notFound(kind, name string) error {
	return fmt.Errorf("registry: %s %q: %w", kind, name, ErrNotFound)
}

func checkName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("registry: %s name is required", kind)
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." || strings.HasPrefix(name, ".") {
		return fmt.Errorf("registry: %s name %q must not contain a path separator or start with a dot", kind, name)
	}
	return nil
}

func sortedTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
