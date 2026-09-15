package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	readyCacheFor = 5 * time.Second
	readyTimeout  = 2 * time.Second
)

type healthBody struct {
	OK   bool   `json:"ok"`
	Hash string `json:"hash"`
}

func (g *Gateway) Ready(ctx context.Context) map[string]error {
	g.mu.Lock()
	if g.probes != nil && g.nowFunc().Sub(g.probed) < readyCacheFor {
		cached := make(map[string]error, len(g.probes))
		for name, err := range g.probes {
			cached[name] = err
		}
		g.mu.Unlock()
		return cached
	}
	g.mu.Unlock()

	probeCtx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()
	names := g.cfg.ServiceNames()
	results := make([]error, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			results[i] = g.probe(probeCtx, name)
		}(i, name)
	}
	wg.Wait()

	probes := make(map[string]error, len(names))
	for i, name := range names {
		probes[name] = results[i]
	}
	g.mu.Lock()
	g.probes = probes
	g.probed = g.nowFunc()
	g.mu.Unlock()

	out := make(map[string]error, len(probes))
	for name, err := range probes {
		out[name] = err
	}
	return out
}

func (g *Gateway) probe(ctx context.Context, name string) error {
	up := g.cfg.Services[name]
	target := strings.TrimRight(up.URL, "/") + "/.bowline/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	resp, err := g.clients[name].Do(req)
	if err != nil {
		return fmt.Errorf("unreachable: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return fmt.Errorf("reading the health response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health answered %d", resp.StatusCode)
	}
	var health healthBody
	if err := json.Unmarshal(body, &health); err != nil {
		return fmt.Errorf("health response is not JSON: %w", err)
	}
	if !health.OK {
		return fmt.Errorf("health reports not ok")
	}
	if strings.HasPrefix(up.Version, "sha256:") && health.Hash != "" && health.Hash != up.Version {
		return fmt.Errorf("serves contract %s but the gateway is pinned to %s", health.Hash, up.Version)
	}
	return nil
}

func (g *Gateway) serveReady(w http.ResponseWriter, req *http.Request) {
	probes := g.Ready(req.Context())
	services := make(map[string]string, len(probes))
	ok := true
	for name, err := range probes {
		if err != nil {
			ok = false
			services[name] = err.Error()
			continue
		}
		services[name] = "ok"
	}
	status := http.StatusOK
	if !ok {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{"ok": ok, "services": services})
}
