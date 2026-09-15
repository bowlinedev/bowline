package registry_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/registry"
	"github.com/bowlinedev/bowline/registry/storetest"
)

func TestMemStoreConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) registry.Store { return registry.NewMemStore() })
}

func TestFileStoreConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) registry.Store {
		store, err := registry.NewFileStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return store
	})
}

func TestFileStoreSurvivesARestart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	first, err := registry.NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.PutService(ctx, registry.Service{Name: "ledger", CreatedAt: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if _, err := first.PutVersion(ctx, registry.Version{Service: "ledger", Hash: "sha256:aaa", PublishedAt: time.Date(2026, 11, 2, 10, 5, 0, 0, time.UTC), Tags: []string{"main"}, Contract: json.RawMessage(`{"bowline":"1.2"}`)}); err != nil {
		t.Fatal(err)
	}
	second, err := registry.NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	tagged, err := second.Tagged(ctx, "ledger", "main")
	if err != nil || tagged.Hash != "sha256:aaa" {
		t.Fatalf("reopened store lost data: %+v %v", tagged, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(tagged.Contract, &doc); err != nil || doc["bowline"] != "1.2" {
		t.Fatalf("reopened store lost the contract: %s %v", tagged.Contract, err)
	}
}

func TestFileStoreIsHumanReadable(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := registry.NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutService(ctx, registry.Service{Name: "ledger", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutVersion(ctx, registry.Version{Service: "ledger", Hash: "sha256:aaa", Contract: json.RawMessage(`{"bowline":"1.2"}`)}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, "services", "ledger", "service.json"),
		filepath.Join(root, "services", "ledger", "versions", "sha256-aaa.json"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !json.Valid(data) || data[len(data)-1] != '\n' {
			t.Fatalf("%s is not pretty JSON ending in a newline", path)
		}
	}
}

func TestFileStoreConcurrentPublishes(t *testing.T) {
	ctx := context.Background()
	store, err := registry.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, service := range []string{"ledger", "billing", "search"} {
		for i := range 8 {
			wg.Add(1)
			go func(service string, i int) {
				defer wg.Done()
				hash := "sha256:" + service + string(rune('a'+i))
				if _, err := store.PutVersion(ctx, registry.Version{Service: service, Hash: hash, PublishedAt: time.Now(), Contract: json.RawMessage(`{}`)}); err != nil {
					t.Error(err)
				}
			}(service, i)
		}
	}
	wg.Wait()
	for _, service := range []string{"ledger", "billing", "search"} {
		list, err := store.Versions(ctx, service)
		if err != nil || len(list) != 8 {
			t.Fatalf("%s: %d versions, %v", service, len(list), err)
		}
	}
}
