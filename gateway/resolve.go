package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bowlinedev/bowline/contract"
)

type Fetcher interface {
	Fetch(ctx context.Context, registryURL, service, version string) ([]byte, error)
}

type HTTPFetcher struct {
	Client *http.Client
}

func (f HTTPFetcher) Fetch(ctx context.Context, registryURL, service, version string) ([]byte, error) {
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	base := strings.TrimRight(registryURL, "/")
	target := base + "/v1/services/" + service + "/versions/" + version
	if !strings.HasPrefix(version, "sha256:") {
		target = base + "/v1/services/" + service + "/latest?tag=" + version
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry %s answered %d for %s", base, resp.StatusCode, service)
	}
	return body, nil
}

func Resolve(ctx context.Context, cfg *Config, client *http.Client) (map[string]*contract.Document, error) {
	return ResolveWith(ctx, cfg, HTTPFetcher{Client: client})
}

func ResolveWith(ctx context.Context, cfg *Config, fetcher Fetcher) (map[string]*contract.Document, error) {
	if cfg == nil {
		return nil, errors.New("gateway: no configuration")
	}
	names := cfg.ServiceNames()
	docs := make(map[string]*contract.Document, len(names))
	type result struct {
		name string
		doc  *contract.Document
		err  error
	}
	results := make([]result, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			doc, err := resolveOne(ctx, cfg, name, fetcher)
			results[i] = result{name: name, doc: doc, err: err}
		}(i, name)
	}
	wg.Wait()

	var problems []string
	for _, r := range results {
		if r.err != nil {
			problems = append(problems, r.err.Error())
			continue
		}
		docs[r.name] = r.doc
	}
	if len(problems) > 0 {
		return nil, errors.New("gateway: " + strings.Join(problems, "; "))
	}
	return docs, nil
}

func resolveOne(ctx context.Context, cfg *Config, name string, fetcher Fetcher) (*contract.Document, error) {
	up := cfg.Services[name]
	var data []byte
	var err error
	switch {
	case up.Contract != "":
		data, err = os.ReadFile(filepath.FromSlash(up.Contract))
		if err != nil {
			return nil, fmt.Errorf("service %q: reading %s: %w", name, up.Contract, err)
		}
	default:
		if fetcher == nil {
			return nil, fmt.Errorf("service %q: no registry fetcher", name)
		}
		data, err = fetcher.Fetch(ctx, up.Registry, name, up.Version)
		if err != nil {
			return nil, fmt.Errorf("service %q: fetching from %s: %w", name, up.Registry, err)
		}
	}
	doc, err := contract.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("service %q: %w", name, err)
	}
	if strings.HasPrefix(up.Version, "sha256:") {
		hash, err := doc.ComputeHash()
		if err != nil {
			return nil, fmt.Errorf("service %q: hashing the contract: %w", name, err)
		}
		if hash != up.Version {
			return nil, fmt.Errorf("service %q is pinned to %s but its contract hashes to %s", name, up.Version, hash)
		}
		doc.Hash = hash
	}
	return doc, nil
}
