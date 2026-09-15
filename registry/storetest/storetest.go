package storetest

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/registry"
)

func Run(t *testing.T, newStore func(t *testing.T) registry.Store) {
	t.Helper()
	cases := map[string]func(*testing.T, registry.Store){
		"services":        services,
		"versions":        versions,
		"tags":            tags,
		"consumers":       consumers,
		"compositions":    compositions,
		"missing records": missing,
		"invalid names":   invalidNames,
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, newStore(t))
		})
	}
}

func at(day int) time.Time {
	return time.Date(2026, 11, day, 10, 0, 0, 0, time.UTC)
}

func sameJSON(t *testing.T, got json.RawMessage, want string) bool {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		t.Fatalf("stored value is not JSON: %s", got)
	}
	if err := json.Unmarshal([]byte(want), &b); err != nil {
		t.Fatalf("expected value is not JSON: %s", want)
	}
	return reflect.DeepEqual(a, b)
}

func services(t *testing.T, store registry.Store) {
	ctx := context.Background()
	empty, err := store.Services(ctx)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty listing: %v %v", empty, err)
	}
	ledger := registry.Service{Name: "ledger", Description: "Invoices and customers", Owners: []string{"team-ledger"}, CreatedAt: at(2)}
	billing := registry.Service{Name: "billing", CreatedAt: at(3)}
	for _, s := range []registry.Service{ledger, billing} {
		if err := store.PutService(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.Service(ctx, "ledger")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, ledger) {
		t.Fatalf("round trip: %+v want %+v", got, ledger)
	}
	list, err := store.Services(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "billing" || list[1].Name != "ledger" {
		t.Fatalf("listing is not sorted by name: %+v", list)
	}
	updated := ledger
	updated.Description = "Invoices"
	if err := store.PutService(ctx, updated); err != nil {
		t.Fatal(err)
	}
	again, err := store.Service(ctx, "ledger")
	if err != nil || again.Description != "Invoices" {
		t.Fatalf("overwrite: %+v %v", again, err)
	}
}

func versions(t *testing.T, store registry.Store) {
	ctx := context.Background()
	first := registry.Version{Service: "ledger", Hash: "sha256:aaa", PublishedAt: at(2), Ref: "git:1", Contract: json.RawMessage(`{"bowline":"1.2"}`)}
	second := registry.Version{Service: "ledger", Hash: "sha256:bbb", PublishedAt: at(4), Ref: "git:2", Contract: json.RawMessage(`{"bowline":"1.2"}`)}
	for _, v := range []registry.Version{first, second} {
		created, err := store.PutVersion(ctx, v)
		if err != nil || !created {
			t.Fatalf("publish %s: created=%v err=%v", v.Hash, created, err)
		}
	}
	created, err := store.PutVersion(ctx, first)
	if err != nil || created {
		t.Fatalf("republishing the same hash must not create: created=%v err=%v", created, err)
	}
	stored, err := store.Version(ctx, "ledger", "sha256:aaa")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Ref != "git:1" || !sameJSON(t, stored.Contract, `{"bowline":"1.2"}`) {
		t.Fatalf("round trip: %+v", stored)
	}
	list, err := store.Versions(ctx, "ledger")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Hash != "sha256:bbb" || list[1].Hash != "sha256:aaa" {
		t.Fatalf("versions are not newest first: %+v", list)
	}
	other, err := store.Versions(ctx, "billing")
	if err != nil || len(other) != 0 {
		t.Fatalf("unknown service listing: %+v %v", other, err)
	}
}

func tags(t *testing.T, store registry.Store) {
	ctx := context.Background()
	first := registry.Version{Service: "ledger", Hash: "sha256:aaa", PublishedAt: at(2), Tags: []string{"main"}, Contract: json.RawMessage(`{}`)}
	second := registry.Version{Service: "ledger", Hash: "sha256:bbb", PublishedAt: at(4), Contract: json.RawMessage(`{}`)}
	for _, v := range []registry.Version{first, second} {
		if _, err := store.PutVersion(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	tagged, err := store.Tagged(ctx, "ledger", "main")
	if err != nil || tagged.Hash != "sha256:aaa" {
		t.Fatalf("tag from publish: %+v %v", tagged, err)
	}
	if err := store.Tag(ctx, "ledger", "main", "sha256:bbb"); err != nil {
		t.Fatal(err)
	}
	tagged, err = store.Tagged(ctx, "ledger", "main")
	if err != nil || tagged.Hash != "sha256:bbb" {
		t.Fatalf("tag reassignment: %+v %v", tagged, err)
	}
	if !reflect.DeepEqual(tagged.Tags, []string{"main"}) {
		t.Fatalf("tags are not reported on the version: %+v", tagged.Tags)
	}
	moved, err := store.Version(ctx, "ledger", "sha256:aaa")
	if err != nil || len(moved.Tags) != 0 {
		t.Fatalf("the old version kept the tag: %+v %v", moved.Tags, err)
	}
	if err := store.Tag(ctx, "ledger", "main", "sha256:missing"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("tagging an unknown hash: %v", err)
	}
}

func consumers(t *testing.T, store registry.Store) {
	ctx := context.Background()
	web := registry.Consumer{Consumer: "web", Provider: "ledger", ProviderHash: "sha256:aaa", RecordedAt: at(3), Usage: json.RawMessage(`{"bowline":"1.2","consumer":"web","interactions":[]}`)}
	mobile := registry.Consumer{Consumer: "mobile", Provider: "ledger", RecordedAt: at(3), Usage: json.RawMessage(`{}`)}
	for _, c := range []registry.Consumer{web, mobile} {
		if err := store.PutConsumer(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	list, err := store.Consumers(ctx, "ledger")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Consumer != "mobile" || list[1].Consumer != "web" {
		t.Fatalf("consumers are not sorted: %+v", list)
	}
	if !sameJSON(t, list[1].Usage, `{"bowline":"1.2","consumer":"web","interactions":[]}`) {
		t.Fatalf("usage round trip: %s", list[1].Usage)
	}
	none, err := store.Consumers(ctx, "billing")
	if err != nil || len(none) != 0 {
		t.Fatalf("unknown provider: %+v %v", none, err)
	}
}

func compositions(t *testing.T, store registry.Store) {
	ctx := context.Background()
	edge := registry.Composition{Gateway: "edge", Hash: "sha256:ccc", Services: map[string]string{"ledger": "sha256:aaa", "billing": "sha256:bbb"}, PublishedAt: at(3)}
	partner := registry.Composition{Gateway: "partner", Hash: "sha256:ddd", Services: map[string]string{"billing": "sha256:bbb"}, PublishedAt: at(4)}
	for _, c := range []registry.Composition{edge, partner} {
		if err := store.PutComposition(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	forLedger, err := store.Compositions(ctx, "ledger")
	if err != nil {
		t.Fatal(err)
	}
	if len(forLedger) != 1 || forLedger[0].Gateway != "edge" || forLedger[0].Services["billing"] != "sha256:bbb" {
		t.Fatalf("compositions for ledger: %+v", forLedger)
	}
	forBilling, err := store.Compositions(ctx, "billing")
	if err != nil {
		t.Fatal(err)
	}
	if len(forBilling) != 2 || forBilling[0].Gateway != "edge" || forBilling[1].Gateway != "partner" {
		t.Fatalf("compositions for billing: %+v", forBilling)
	}
	none, err := store.Compositions(ctx, "unknown")
	if err != nil || len(none) != 0 {
		t.Fatalf("unknown service: %+v %v", none, err)
	}
}

func missing(t *testing.T, store registry.Store) {
	ctx := context.Background()
	if _, err := store.Service(ctx, "nope"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("service: %v", err)
	}
	if _, err := store.Version(ctx, "nope", "sha256:zzz"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("version: %v", err)
	}
	if _, err := store.Tagged(ctx, "nope", "main"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("tag: %v", err)
	}
}

func invalidNames(t *testing.T, store registry.Store) {
	ctx := context.Background()
	if err := store.PutService(ctx, registry.Service{Name: "../escape"}); err == nil {
		t.Fatal("a service name with a path separator must be rejected")
	}
	if err := store.PutService(ctx, registry.Service{}); err == nil {
		t.Fatal("an empty service name must be rejected")
	}
	if _, err := store.PutVersion(ctx, registry.Version{Service: "ledger"}); err == nil {
		t.Fatal("a version without a hash must be rejected")
	}
	if err := store.PutConsumer(ctx, registry.Consumer{Consumer: "a/b", Provider: "ledger"}); err == nil {
		t.Fatal("a consumer name with a path separator must be rejected")
	}
}
