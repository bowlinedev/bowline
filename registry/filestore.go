package registry

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
)

type FileStore struct {
	root string
	mu   sync.Mutex
}

func NewFileStore(root string) (*FileStore, error) {
	if root == "" {
		return nil, errors.New("registry: the file store needs a root directory")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &FileStore{root: root}, nil
}

func hashFile(hash string) string {
	return strings.ReplaceAll(hash, ":", "-") + ".json"
}

func (f *FileStore) servicePath(name string, parts ...string) string {
	return filepath.Join(append([]string{f.root, "services", name}, parts...)...)
}

func (f *FileStore) gatewayPath(name string, parts ...string) string {
	return filepath.Join(append([]string{f.root, "gateways", name}, parts...)...)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (f *FileStore) PutService(ctx context.Context, s Service) error {
	if err := checkName("service", s.Name); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return writeJSON(f.servicePath(s.Name, "service.json"), s)
}

func (f *FileStore) Services(ctx context.Context) ([]Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(f.root, "services"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Service
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var s Service
		if err := readJSON(f.servicePath(e.Name(), "service.json"), &s); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *FileStore) Service(ctx context.Context, name string) (Service, error) {
	if err := checkName("service", name); err != nil {
		return Service{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var s Service
	if err := readJSON(f.servicePath(name, "service.json"), &s); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Service{}, notFound("service", name)
		}
		return Service{}, err
	}
	return s, nil
}

func (f *FileStore) PutVersion(ctx context.Context, v Version) (bool, error) {
	if err := checkName("service", v.Service); err != nil {
		return false, err
	}
	if v.Hash == "" {
		return false, errors.New("registry: a version needs a hash")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	path := f.servicePath(v.Service, "versions", hashFile(v.Hash))
	_, err := os.Stat(path)
	created := errors.Is(err, os.ErrNotExist)
	if err != nil && !created {
		return false, err
	}
	if created {
		stored := v
		stored.Tags = nil
		if err := writeJSON(path, stored); err != nil {
			return false, err
		}
	}
	for _, tag := range sortedTags(v.Tags) {
		if err := f.tag(v.Service, tag, v.Hash); err != nil {
			return created, err
		}
	}
	return created, nil
}

func (f *FileStore) tag(service, tag, hash string) error {
	if err := checkName("tag", tag); err != nil {
		return err
	}
	if _, err := os.Stat(f.servicePath(service, "versions", hashFile(hash))); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return notFound("version", service+"@"+hash)
		}
		return err
	}
	return writeJSON(f.servicePath(service, "tags", tag+".json"), map[string]string{"hash": hash})
}

func (f *FileStore) tagsFor(service string) (map[string][]string, error) {
	entries, err := os.ReadDir(f.servicePath(service, "tags"))
	if errors.Is(err, os.ErrNotExist) {
		return map[string][]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var record map[string]string
		if err := readJSON(f.servicePath(service, "tags", e.Name()), &record); err != nil {
			return nil, err
		}
		tag := strings.TrimSuffix(e.Name(), ".json")
		out[record["hash"]] = append(out[record["hash"]], tag)
	}
	for hash := range out {
		slices.Sort(out[hash])
	}
	return out, nil
}

func (f *FileStore) Versions(ctx context.Context, service string) ([]Version, error) {
	if err := checkName("service", service); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.versions(service)
}

func (f *FileStore) versions(service string) ([]Version, error) {
	entries, err := os.ReadDir(f.servicePath(service, "versions"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tags, err := f.tagsFor(service)
	if err != nil {
		return nil, err
	}
	var out []Version
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var v Version
		if err := readJSON(f.servicePath(service, "versions", e.Name()), &v); err != nil {
			return nil, err
		}
		v.Tags = tags[v.Hash]
		out = append(out, v)
	}
	sortVersions(out)
	return out, nil
}

func (f *FileStore) Version(ctx context.Context, service, hash string) (Version, error) {
	if err := checkName("service", service); err != nil {
		return Version{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var v Version
	if err := readJSON(f.servicePath(service, "versions", hashFile(hash)), &v); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Version{}, notFound("version", service+"@"+hash)
		}
		return Version{}, err
	}
	tags, err := f.tagsFor(service)
	if err != nil {
		return Version{}, err
	}
	v.Tags = tags[v.Hash]
	return v, nil
}

func (f *FileStore) Tag(ctx context.Context, service, tag, hash string) error {
	if err := checkName("service", service); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tag(service, tag, hash)
}

func (f *FileStore) Tagged(ctx context.Context, service, tag string) (Version, error) {
	if err := checkName("service", service); err != nil {
		return Version{}, err
	}
	if err := checkName("tag", tag); err != nil {
		return Version{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var record map[string]string
	if err := readJSON(f.servicePath(service, "tags", tag+".json"), &record); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Version{}, notFound("tag", service+":"+tag)
		}
		return Version{}, err
	}
	var v Version
	if err := readJSON(f.servicePath(service, "versions", hashFile(record["hash"])), &v); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Version{}, notFound("version", service+"@"+record["hash"])
		}
		return Version{}, err
	}
	tags, err := f.tagsFor(service)
	if err != nil {
		return Version{}, err
	}
	v.Tags = tags[v.Hash]
	return v, nil
}

func (f *FileStore) PutConsumer(ctx context.Context, c Consumer) error {
	if err := checkName("provider", c.Provider); err != nil {
		return err
	}
	if err := checkName("consumer", c.Consumer); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return writeJSON(f.servicePath(c.Provider, "consumers", c.Consumer+".json"), c)
}

func (f *FileStore) Consumers(ctx context.Context, provider string) ([]Consumer, error) {
	if err := checkName("provider", provider); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	entries, err := os.ReadDir(f.servicePath(provider, "consumers"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Consumer
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var c Consumer
		if err := readJSON(f.servicePath(provider, "consumers", e.Name()), &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Consumer < out[j].Consumer })
	return out, nil
}

func (f *FileStore) PutComposition(ctx context.Context, c Composition) error {
	if err := checkName("gateway", c.Gateway); err != nil {
		return err
	}
	if c.Hash == "" {
		return errors.New("registry: a composition needs a hash")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return writeJSON(f.gatewayPath(c.Gateway, "compositions", hashFile(c.Hash)), c)
}

func (f *FileStore) Compositions(ctx context.Context, service string) ([]Composition, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	gateways, err := os.ReadDir(filepath.Join(f.root, "gateways"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Composition
	for _, g := range gateways {
		if !g.IsDir() {
			continue
		}
		entries, err := os.ReadDir(f.gatewayPath(g.Name(), "compositions"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			var c Composition
			if err := readJSON(f.gatewayPath(g.Name(), "compositions", e.Name()), &c); err != nil {
				return nil, err
			}
			if service != "" {
				if _, ok := c.Services[service]; !ok {
					continue
				}
			}
			out = append(out, c)
		}
	}
	sortCompositions(out)
	return out, nil
}

func sortVersions(v []Version) {
	sort.Slice(v, func(i, j int) bool {
		if !v[i].PublishedAt.Equal(v[j].PublishedAt) {
			return v[i].PublishedAt.After(v[j].PublishedAt)
		}
		return v[i].Hash < v[j].Hash
	})
}

func sortCompositions(c []Composition) {
	sort.Slice(c, func(i, j int) bool {
		if c[i].Gateway != c[j].Gateway {
			return c[i].Gateway < c[j].Gateway
		}
		if !c[i].PublishedAt.Equal(c[j].PublishedAt) {
			return c[i].PublishedAt.After(c[j].PublishedAt)
		}
		return c[i].Hash < c[j].Hash
	})
}
