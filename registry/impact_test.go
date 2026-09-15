package registry_test

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/registry"
)

func enumRef(id string) *contract.Type {
	return &contract.Type{Kind: contract.Ref, ID: id}
}

func statusEnum(values ...string) *contract.TypeDecl {
	decl := &contract.TypeDecl{Kind: contract.Enum, Name: "Status", Base: "string"}
	for _, v := range values {
		decl.Values = append(decl.Values, contract.EnumValue{Name: v, Value: v})
	}
	return decl
}

func ledgerBaseline(t *testing.T) *contract.Document {
	t.Helper()
	doc := document(t,
		query("invoices.list", object(
			field("items", &contract.Type{Kind: contract.Array, Elem: object(
				field("id", str()),
				field("total", str()),
				field("status", enumRef("app.Status")),
			)}),
		)),
		&contract.Procedure{
			Path: "invoices.search", Kind: "query", Method: "GET",
			Input:  object(field("query", str())),
			Output: object(field("hits", str())),
		},
	)
	doc.Types["app.Status"] = statusEnum("draft", "sent")
	return doc
}

func webUsage() json.RawMessage {
	return json.RawMessage(`{
	  "bowline": "1.2",
	  "consumer": "web",
	  "provider": "ledger",
	  "interactions": [
	    {"procedure": "invoices.list", "method": "GET", "input": {},
	     "response": {"status": 200, "body": {"items": [{"id": "1", "total": "USD 1.00", "status": "sent"}]}}}
	  ]
	}`)
}

func mobileUsage() json.RawMessage {
	return json.RawMessage(`{
	  "bowline": "1.2",
	  "consumer": "mobile",
	  "provider": "ledger",
	  "interactions": [
	    {"procedure": "invoices.list", "method": "GET", "input": {},
	     "response": {"status": 200, "body": {"items": [{"id": "1"}]}}}
	  ]
	}`)
}

func seedLedger(t *testing.T, store registry.Store, consumers map[string]json.RawMessage) {
	t.Helper()
	ctx := context.Background()
	baseline := ledgerBaseline(t)
	raw, err := baseline.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := baseline.ComputeHash()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutService(ctx, registry.Service{Name: "ledger"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutVersion(ctx, registry.Version{Service: "ledger", Hash: hash, Contract: raw, Tags: []string{"main"}}); err != nil {
		t.Fatal(err)
	}
	for name, usage := range consumers {
		if err := store.PutConsumer(ctx, registry.Consumer{Consumer: name, Provider: "ledger", Usage: usage}); err != nil {
			t.Fatal(err)
		}
	}
}

func reasonsFor(report *registry.ImpactReport, consumer string) []string {
	var out []string
	for _, a := range report.Affected {
		if a.Consumer == consumer {
			out = append(out, a.Reason)
		}
	}
	return out
}

func TestImpactWithoutABaseline(t *testing.T) {
	store := registry.NewMemStore()
	report, err := registry.Impact(context.Background(), store, "ledger", ledgerBaseline(t), false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Baseline != "" || !report.OK || len(report.Changes) != 0 {
		t.Fatalf("%+v", report)
	}
	if !strings.Contains(registry.FormatText(report), "no baseline") {
		t.Fatalf("text: %s", registry.FormatText(report))
	}
}

func TestImpactRemovedFieldSplitsConsumers(t *testing.T) {
	store := registry.NewMemStore()
	seedLedger(t, store, map[string]json.RawMessage{"web": webUsage(), "mobile": mobileUsage()})
	candidate := ledgerBaseline(t)
	elem := candidate.Procedures[0].Output.Fields[0].Type.Elem
	elem.Fields = []*contract.Field{elem.Fields[0], elem.Fields[2]}
	report, err := registry.Impact(context.Background(), store, "ledger", candidate, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("removing a read field must break: %+v", report)
	}
	if got := reasonsFor(report, "web"); len(got) != 1 || got[0] != "reads items.*.total" {
		t.Fatalf("web: %+v", report.Affected)
	}
	if got := reasonsFor(report, "mobile"); len(got) != 0 {
		t.Fatalf("mobile does not read total: %+v", got)
	}
	if len(report.Unattributed) != 0 {
		t.Fatalf("unattributed: %+v", report.Unattributed)
	}
}

func TestImpactUnusedFieldIsUnattributed(t *testing.T) {
	ctx := context.Background()
	store := registry.NewMemStore()
	seedLedger(t, store, map[string]json.RawMessage{"mobile": mobileUsage()})
	candidate := ledgerBaseline(t)
	elem := candidate.Procedures[0].Output.Fields[0].Type.Elem
	elem.Fields = []*contract.Field{elem.Fields[0], elem.Fields[2]}
	lenient, err := registry.Impact(ctx, store, "ledger", candidate, false)
	if err != nil {
		t.Fatal(err)
	}
	if !lenient.OK || len(lenient.Affected) != 0 || len(lenient.Unattributed) != 1 {
		t.Fatalf("lenient: %+v", lenient)
	}
	strict, err := registry.Impact(ctx, store, "ledger", candidate, true)
	if err != nil {
		t.Fatal(err)
	}
	if strict.OK {
		t.Fatalf("strict must fail on unattributed breaks: %+v", strict)
	}
}

func TestImpactRemovedProcedure(t *testing.T) {
	store := registry.NewMemStore()
	seedLedger(t, store, map[string]json.RawMessage{"web": webUsage()})
	candidate := ledgerBaseline(t)
	candidate.Procedures = candidate.Procedures[1:]
	report, err := registry.Impact(context.Background(), store, "ledger", candidate, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := reasonsFor(report, "web"); len(got) != 1 || got[0] != "calls invoices.list" {
		t.Fatalf("%+v", report.Affected)
	}
}

func TestImpactRequiredInputFieldAdded(t *testing.T) {
	store := registry.NewMemStore()
	usage := json.RawMessage(`{"bowline":"1.2","consumer":"web","interactions":[
	  {"procedure":"invoices.search","method":"GET","input":{"query":"ada"},"response":{"status":200,"body":{"hits":"1"}}}]}`)
	seedLedger(t, store, map[string]json.RawMessage{"web": usage})
	candidate := ledgerBaseline(t)
	search := candidate.Procedures[1]
	search.Input.Fields = append(search.Input.Fields, &contract.Field{Name: "tenant", Type: str()})
	report, err := registry.Impact(context.Background(), store, "ledger", candidate, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := reasonsFor(report, "web"); len(got) != 1 || got[0] != "calls invoices.search" {
		t.Fatalf("%+v", report.Affected)
	}
}

func TestImpactRemovedEnumValue(t *testing.T) {
	store := registry.NewMemStore()
	seedLedger(t, store, map[string]json.RawMessage{"web": webUsage(), "mobile": mobileUsage()})
	candidate := ledgerBaseline(t)
	candidate.Types["app.Status"] = statusEnum("draft")
	report, err := registry.Impact(context.Background(), store, "ledger", candidate, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := reasonsFor(report, "web"); len(got) != 1 || got[0] != "reads items.*.status" {
		t.Fatalf("web: %+v", report.Affected)
	}
	if got := reasonsFor(report, "mobile"); len(got) != 0 {
		t.Fatalf("mobile does not read status: %+v", got)
	}
}

func TestImpactThroughAGateway(t *testing.T) {
	ctx := context.Background()
	store := registry.NewMemStore()
	seedLedger(t, store, nil)
	baseline, err := store.Tagged(ctx, "ledger", "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutComposition(ctx, registry.Composition{
		Gateway:  "edge",
		Hash:     "sha256:composed",
		Services: map[string]string{"ledger": baseline.Hash},
	}); err != nil {
		t.Fatal(err)
	}
	usage := json.RawMessage(`{"bowline":"1.2","consumer":"partner","interactions":[
	  {"procedure":"ledger.invoices.list","method":"GET","input":{},
	   "response":{"status":200,"body":{"items":[{"id":"1","total":"USD 1.00"}]}}}]}`)
	if err := store.PutConsumer(ctx, registry.Consumer{Consumer: "partner", Provider: "edge", Usage: usage}); err != nil {
		t.Fatal(err)
	}
	candidate := ledgerBaseline(t)
	elem := candidate.Procedures[0].Output.Fields[0].Type.Elem
	elem.Fields = []*contract.Field{elem.Fields[0], elem.Fields[2]}

	skipped, err := registry.Impact(ctx, store, "ledger", candidate, false)
	if err != nil {
		t.Fatal(err)
	}
	if !skipped.SkippedCompositions || len(skipped.Affected) != 0 {
		t.Fatalf("without a composer the gateway must be skipped: %+v", skipped)
	}

	report, err := registry.ImpactWith(ctx, store, "ledger", candidate, registry.ImpactOptions{Composer: prefixCompose})
	if err != nil {
		t.Fatal(err)
	}
	if report.SkippedCompositions {
		t.Fatal("a composer was supplied")
	}
	if len(report.Affected) != 1 {
		t.Fatalf("gateway consumer: %+v", report.Affected)
	}
	hit := report.Affected[0]
	if hit.Consumer != "partner" || hit.Via != "edge" || hit.Reason != "reads items.*.total" {
		t.Fatalf("attribution: %+v", hit)
	}
	if !strings.Contains(hit.Change.Path, "procedure invoices.list") {
		t.Fatalf("the report must name the service's own change: %q", hit.Change.Path)
	}
	if len(report.Unattributed) != 0 {
		t.Fatalf("the change is attributed through the gateway: %+v", report.Unattributed)
	}
}

func prefixCompose(services map[string]*contract.Document) (*contract.Document, error) {
	out := &contract.Document{
		Bowline: contract.Version,
		Types:   map[string]*contract.TypeDecl{},
		Errors:  map[string]*contract.ErrorDecl{},
	}
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		doc := services[name]
		for id, decl := range doc.Types {
			out.Types[name+":"+id] = decl
		}
		for id, decl := range doc.Errors {
			out.Errors[name+":"+id] = decl
		}
		for _, p := range doc.Procedures {
			copied := *p
			copied.Path = name + "." + p.Path
			copied.Input = rewriteRefs(p.Input, name)
			copied.Output = rewriteRefs(p.Output, name)
			out.Procedures = append(out.Procedures, &copied)
		}
	}
	return out, nil
}

func rewriteRefs(t *contract.Type, service string) *contract.Type {
	if t == nil {
		return nil
	}
	copied := *t
	if copied.Kind == contract.Ref && copied.ID != "" {
		copied.ID = service + ":" + copied.ID
	}
	copied.Elem = rewriteRefs(t.Elem, service)
	copied.Key = rewriteRefs(t.Key, service)
	copied.Value = rewriteRefs(t.Value, service)
	if len(t.Args) > 0 {
		copied.Args = make([]*contract.Type, len(t.Args))
		for i, a := range t.Args {
			copied.Args[i] = rewriteRefs(a, service)
		}
	}
	if len(t.Fields) > 0 {
		copied.Fields = make([]*contract.Field, len(t.Fields))
		for i, f := range t.Fields {
			field := *f
			field.Type = rewriteRefs(f.Type, service)
			copied.Fields[i] = &field
		}
	}
	return &copied
}

func TestImpactIsDeterministic(t *testing.T) {
	ctx := context.Background()
	store := registry.NewMemStore()
	seedLedger(t, store, map[string]json.RawMessage{"web": webUsage(), "mobile": mobileUsage()})
	candidate := ledgerBaseline(t)
	elem := candidate.Procedures[0].Output.Fields[0].Type.Elem
	elem.Fields = []*contract.Field{elem.Fields[2]}
	var first string
	for range 5 {
		report, err := registry.Impact(ctx, store, "ledger", candidate, false)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		if first == "" {
			first = string(data)
			continue
		}
		if string(data) != first {
			t.Fatalf("report is not deterministic:\n%s\n%s", first, data)
		}
	}
}

func TestImpactOverHTTP(t *testing.T) {
	ctx := context.Background()
	client, store := newServer(t)
	seedLedger(t, store, map[string]json.RawMessage{"web": webUsage()})
	candidate := ledgerBaseline(t)
	elem := candidate.Procedures[0].Output.Fields[0].Type.Elem
	elem.Fields = []*contract.Field{elem.Fields[0], elem.Fields[2]}
	report, err := client.Impact(ctx, "ledger", candidate, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || len(report.Affected) != 1 || report.Affected[0].Consumer != "web" {
		t.Fatalf("%+v", report)
	}
	result, err := client.Publish(ctx, "ledger", candidate, "git:2", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Impact == nil || result.Impact.OK {
		t.Fatalf("publishing must report impact: %+v", result.Impact)
	}
	if !strings.Contains(registry.FormatText(report), "breaks    web") {
		t.Fatalf("text: %s", registry.FormatText(report))
	}
}
