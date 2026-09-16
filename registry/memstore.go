package registry

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"sort"
	"sync"
)

type MemStore struct {
	mu           sync.Mutex
	services     map[string]Service
	versions     map[string]map[string]Version
	tags         map[string]map[string]string
	consumers    map[string]map[string]Consumer
	compositions map[string]map[string]Composition
}

func NewMemStore() *MemStore {
	return &MemStore{
		services:     map[string]Service{},
		versions:     map[string]map[string]Version{},
		tags:         map[string]map[string]string{},
		consumers:    map[string]map[string]Consumer{},
		compositions: map[string]map[string]Composition{},
	}
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}

func (m *MemStore) PutService(ctx context.Context, s Service) error {
	if err := checkName("service", s.Name); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s.Owners = append([]string(nil), s.Owners...)
	m.services[s.Name] = s
	return nil
}

func (m *MemStore) Services(ctx context.Context) ([]Service, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Service, 0, len(m.services))
	for _, s := range m.services {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func (m *MemStore) Service(ctx context.Context, name string) (Service, error) {
	if err := checkName("service", name); err != nil {
		return Service{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.services[name]
	if !ok {
		return Service{}, notFound("service", name)
	}
	return s, nil
}

func (m *MemStore) PutVersion(ctx context.Context, v Version) (bool, error) {
	if err := checkName("service", v.Service); err != nil {
		return false, err
	}
	if v.Hash == "" {
		return false, errors.New("registry: a version needs a hash")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	byHash := m.versions[v.Service]
	if byHash == nil {
		byHash = map[string]Version{}
		m.versions[v.Service] = byHash
	}
	_, exists := byHash[v.Hash]
	if !exists {
		stored := v
		stored.Tags = nil
		stored.Contract = cloneRaw(v.Contract)
		byHash[v.Hash] = stored
	}
	for _, tag := range sortedTags(v.Tags) {
		if err := m.tag(v.Service, tag, v.Hash); err != nil {
			return !exists, err
		}
	}
	return !exists, nil
}

func (m *MemStore) tag(service, tag, hash string) error {
	if err := checkName("tag", tag); err != nil {
		return err
	}
	if _, ok := m.versions[service][hash]; !ok {
		return notFound("version", service+"@"+hash)
	}
	byTag := m.tags[service]
	if byTag == nil {
		byTag = map[string]string{}
		m.tags[service] = byTag
	}
	byTag[tag] = hash
	return nil
}

func (m *MemStore) tagsFor(service string) map[string][]string {
	out := map[string][]string{}
	for tag, hash := range m.tags[service] {
		out[hash] = append(out[hash], tag)
	}
	for hash := range out {
		sort.Strings(out[hash])
	}
	return out
}

func (m *MemStore) Versions(ctx context.Context, service string) ([]Version, error) {
	if err := checkName("service", service); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tags := m.tagsFor(service)
	var out []Version
	for _, v := range m.versions[service] {
		v.Tags = tags[v.Hash]
		v.Contract = cloneRaw(v.Contract)
		out = append(out, v)
	}
	sortVersions(out)
	return out, nil
}

func (m *MemStore) Version(ctx context.Context, service, hash string) (Version, error) {
	if err := checkName("service", service); err != nil {
		return Version{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.versions[service][hash]
	if !ok {
		return Version{}, notFound("version", service+"@"+hash)
	}
	v.Tags = m.tagsFor(service)[v.Hash]
	v.Contract = cloneRaw(v.Contract)
	return v, nil
}

func (m *MemStore) Tag(ctx context.Context, service, tag, hash string) error {
	if err := checkName("service", service); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tag(service, tag, hash)
}

func (m *MemStore) Tagged(ctx context.Context, service, tag string) (Version, error) {
	if err := checkName("service", service); err != nil {
		return Version{}, err
	}
	if err := checkName("tag", tag); err != nil {
		return Version{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	hash, ok := m.tags[service][tag]
	if !ok {
		return Version{}, notFound("tag", service+":"+tag)
	}
	v, ok := m.versions[service][hash]
	if !ok {
		return Version{}, notFound("version", service+"@"+hash)
	}
	v.Tags = m.tagsFor(service)[v.Hash]
	v.Contract = cloneRaw(v.Contract)
	return v, nil
}

func (m *MemStore) PutConsumer(ctx context.Context, c Consumer) error {
	if err := checkName("provider", c.Provider); err != nil {
		return err
	}
	if err := checkName("consumer", c.Consumer); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	byName := m.consumers[c.Provider]
	if byName == nil {
		byName = map[string]Consumer{}
		m.consumers[c.Provider] = byName
	}
	c.Usage = cloneRaw(c.Usage)
	byName[c.Consumer] = c
	return nil
}

func (m *MemStore) Consumers(ctx context.Context, provider string) ([]Consumer, error) {
	if err := checkName("provider", provider); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Consumer
	for _, c := range m.consumers[provider] {
		c.Usage = cloneRaw(c.Usage)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Consumer < out[j].Consumer })
	return out, nil
}

func (m *MemStore) PutComposition(ctx context.Context, c Composition) error {
	if err := checkName("gateway", c.Gateway); err != nil {
		return err
	}
	if c.Hash == "" {
		return errors.New("registry: a composition needs a hash")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	byHash := m.compositions[c.Gateway]
	if byHash == nil {
		byHash = map[string]Composition{}
		m.compositions[c.Gateway] = byHash
	}
	services := map[string]string{}
	maps.Copy(services, c.Services)
	c.Services = services
	byHash[c.Hash] = c
	return nil
}

func (m *MemStore) Compositions(ctx context.Context, service string) ([]Composition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Composition
	for _, byHash := range m.compositions {
		for _, c := range byHash {
			if service != "" {
				if _, ok := c.Services[service]; !ok {
					continue
				}
			}
			services := map[string]string{}
			maps.Copy(services, c.Services)
			c.Services = services
			out = append(out, c)
		}
	}
	sortCompositions(out)
	return out, nil
}
