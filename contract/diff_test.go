package contract

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func prim(name string) *Type {
	return &Type{Kind: Primitive, Name: name}
}

func ref(id string, args ...*Type) *Type {
	return &Type{Kind: Ref, ID: id, Args: args}
}

func field(name string, t *Type) *Field {
	return &Field{Name: name, Type: t}
}

func structDecl(name string, fields ...*Field) *TypeDecl {
	return &TypeDecl{Kind: Struct, Name: name, Fields: fields}
}

func doc(types map[string]*TypeDecl, errs map[string]*ErrorDecl, procs ...*Procedure) *Document {
	if types == nil {
		types = map[string]*TypeDecl{}
	}
	if errs == nil {
		errs = map[string]*ErrorDecl{}
	}
	return &Document{Bowline: Version, Types: types, Errors: errs, Procedures: procs}
}

func query(path string, in, out *Type) *Procedure {
	return &Procedure{Path: path, Kind: "query", Method: "GET", Input: in, Output: out}
}

func single(in, out *TypeDecl) *Document {
	return doc(map[string]*TypeDecl{"p.In": in, "p.Out": out}, nil, query("get", ref("p.In"), ref("p.Out")))
}

func TestDiffRulesTable(t *testing.T) {
	base := func() (*TypeDecl, *TypeDecl) {
		return structDecl("In", field("id", prim("int64"))), structDecl("Out", field("name", prim("string")))
	}
	cases := []struct {
		name string
		old  *Document
		new  *Document
		want Changes
	}{
		{
			name: "procedure removed",
			old:  single(base()),
			new:  doc(nil, nil),
			want: Changes{{"procedure get", Breaking, "procedure removed"}},
		},
		{
			name: "procedure added",
			old:  doc(nil, nil),
			new:  single(base()),
			want: Changes{{"procedure get", Added, "procedure added"}},
		},
		{
			name: "kind changed",
			old:  single(base()),
			new: func() *Document {
				d := single(base())
				d.Procedures[0].Kind = "mutation"
				d.Procedures[0].Method = "POST"
				return d
			}(),
			want: Changes{{"procedure get", Breaking, "kind changed from query to mutation"}},
		},
		{
			name: "method changed",
			old:  single(base()),
			new: func() *Document {
				d := single(base())
				d.Procedures[0].Method = "POST"
				return d
			}(),
			want: Changes{{"procedure get", Breaking, "method changed from GET to POST"}},
		},
		{
			name: "required field added",
			old:  single(base()),
			new: single(structDecl("In", field("id", prim("int64")), field("name", prim("string"))),
				structDecl("Out", field("name", prim("string")), field("age", prim("int32")))),
			want: Changes{
				{"procedure get input field name", Breaking, "required field added"},
				{"procedure get output field age", Added, "field added"},
			},
		},
		{
			name: "optional field added",
			old:  single(base()),
			new: single(structDecl("In", field("id", prim("int64")), &Field{Name: "name", Type: prim("string"), Optional: true}),
				structDecl("Out", field("name", prim("string")), &Field{Name: "age", Type: prim("int32"), Optional: true})),
			want: Changes{
				{"procedure get input field name", Widened, "optional field added"},
				{"procedure get output field age", Added, "field added"},
			},
		},
		{
			name: "field removed",
			old:  single(base()),
			new:  single(structDecl("In"), structDecl("Out")),
			want: Changes{
				{"procedure get input field id", Widened, "field removed"},
				{"procedure get output field name", Breaking, "field removed"},
			},
		},
		{
			name: "optional to required",
			old: single(structDecl("In", &Field{Name: "id", Type: prim("int64"), Optional: true}),
				structDecl("Out", &Field{Name: "name", Type: prim("string"), Optional: true})),
			new: single(base()),
			want: Changes{
				{"procedure get input field id", Breaking, "field is now required"},
				{"procedure get output field name", Narrowed, "field is now always present"},
			},
		},
		{
			name: "required to optional",
			old:  single(base()),
			new: single(structDecl("In", &Field{Name: "id", Type: prim("int64"), Optional: true}),
				structDecl("Out", &Field{Name: "name", Type: prim("string"), Optional: true})),
			want: Changes{
				{"procedure get input field id", Widened, "field is now optional"},
				{"procedure get output field name", Breaking, "field may now be absent"},
			},
		},
		{
			name: "nullable added",
			old:  single(base()),
			new: single(structDecl("In", &Field{Name: "id", Type: prim("int64"), Nullable: true}),
				structDecl("Out", &Field{Name: "name", Type: prim("string"), Nullable: true})),
			want: Changes{
				{"procedure get input field id", Widened, "field now accepts null"},
				{"procedure get output field name", Breaking, "field may now be null"},
			},
		},
		{
			name: "nullable removed",
			old: single(structDecl("In", &Field{Name: "id", Type: prim("int64"), Nullable: true}),
				structDecl("Out", &Field{Name: "name", Type: prim("string"), Nullable: true})),
			new: single(base()),
			want: Changes{
				{"procedure get input field id", Breaking, "field no longer accepts null"},
				{"procedure get output field name", Narrowed, "field is no longer null"},
			},
		},
		{
			name: "primitive changed",
			old:  single(base()),
			new:  single(structDecl("In", field("id", prim("string"))), structDecl("Out", field("name", prim("int64")))),
			want: Changes{
				{"procedure get input field id", Breaking, "type changed from int64 to string"},
				{"procedure get output field name", Breaking, "type changed from string to int64"},
			},
		},
		{
			name: "enum value added",
			old: doc(map[string]*TypeDecl{
				"p.In":     structDecl("In", field("s", ref("p.Status"))),
				"p.Out":    structDecl("Out", field("s", ref("p.Status"))),
				"p.Status": {Kind: Enum, Name: "Status", Base: "string", Values: []EnumValue{{"A", "a"}}},
			}, nil, query("get", ref("p.In"), ref("p.Out"))),
			new: doc(map[string]*TypeDecl{
				"p.In":     structDecl("In", field("s", ref("p.Status"))),
				"p.Out":    structDecl("Out", field("s", ref("p.Status"))),
				"p.Status": {Kind: Enum, Name: "Status", Base: "string", Values: []EnumValue{{"A", "a"}, {"B", "b"}}},
			}, nil, query("get", ref("p.In"), ref("p.Out"))),
			want: Changes{
				{"procedure get input field s", Widened, "enum value b added"},
				{"procedure get output field s", Breaking, "enum value b added"},
			},
		},
		{
			name: "enum value removed",
			old: doc(map[string]*TypeDecl{
				"p.In":     structDecl("In", field("s", ref("p.Status"))),
				"p.Out":    structDecl("Out", field("s", ref("p.Status"))),
				"p.Status": {Kind: Enum, Name: "Status", Base: "string", Values: []EnumValue{{"A", "a"}, {"B", "b"}}},
			}, nil, query("get", ref("p.In"), ref("p.Out"))),
			new: doc(map[string]*TypeDecl{
				"p.In":     structDecl("In", field("s", ref("p.Status"))),
				"p.Out":    structDecl("Out", field("s", ref("p.Status"))),
				"p.Status": {Kind: Enum, Name: "Status", Base: "string", Values: []EnumValue{{"A", "a"}}},
			}, nil, query("get", ref("p.In"), ref("p.Out"))),
			want: Changes{
				{"procedure get input field s", Breaking, "enum value b removed"},
				{"procedure get output field s", Narrowed, "enum value b removed"},
			},
		},
		{
			name: "validation rule tightened",
			old: single(structDecl("In", &Field{Name: "id", Type: prim("int64"), Rules: []Rule{{"min", "1"}}}),
				structDecl("Out", &Field{Name: "name", Type: prim("string"), Rules: []Rule{{"max", "10"}}})),
			new: single(structDecl("In", &Field{Name: "id", Type: prim("int64"), Rules: []Rule{{"min", "5"}, {"max", "9"}}}),
				structDecl("Out", &Field{Name: "name", Type: prim("string"), Rules: []Rule{{"max", "5"}}})),
			want: Changes{
				{"procedure get input field id", Breaking, "validation rule min changed from 1 to 5"},
				{"procedure get input field id", Breaking, "validation rule max added"},
				{"procedure get output field name", Narrowed, "validation rule max changed from 10 to 5"},
			},
		},
		{
			name: "validation rule loosened",
			old: single(structDecl("In", &Field{Name: "id", Type: prim("int64"), Rules: []Rule{{"min", "5"}, {"required", ""}}}),
				structDecl("Out", &Field{Name: "name", Type: prim("string"), Rules: []Rule{{"max", "5"}}})),
			new: single(structDecl("In", &Field{Name: "id", Type: prim("int64"), Rules: []Rule{{"min", "1"}}}),
				structDecl("Out", &Field{Name: "name", Type: prim("string"), Rules: []Rule{{"max", "10"}}})),
			want: Changes{
				{"procedure get input field id", Widened, "validation rule min changed from 5 to 1"},
				{"procedure get input field id", Widened, "validation rule required removed"},
				{"procedure get output field name", Added, "validation rule max changed from 5 to 10"},
			},
		},
		{
			name: "error variant added and removed",
			old: doc(map[string]*TypeDecl{"p.In": structDecl("In"), "p.Out": structDecl("Out")},
				map[string]*ErrorDecl{"p.Gone": {Name: "Gone", Code: "NOT_FOUND"}},
				&Procedure{Path: "get", Kind: "query", Method: "GET", Input: ref("p.In"), Output: ref("p.Out"), Errors: []string{"p.Gone"}}),
			new: doc(map[string]*TypeDecl{"p.In": structDecl("In"), "p.Out": structDecl("Out")},
				map[string]*ErrorDecl{"p.Locked": {Name: "Locked", Code: "FAILED_PRECONDITION"}},
				&Procedure{Path: "get", Kind: "query", Method: "GET", Input: ref("p.In"), Output: ref("p.Out"), Errors: []string{"p.Locked"}}),
			want: Changes{
				{"procedure get error Gone", Narrowed, "error variant removed"},
				{"procedure get error Locked", Widened, "error variant added"},
			},
		},
		{
			name: "idempotent removed",
			old: func() *Document {
				d := single(base())
				d.Procedures[0].Idempotent = true
				return d
			}(),
			new:  single(base()),
			want: Changes{{"procedure get", Breaking, "idempotency removed"}},
		},
		{
			name: "deprecated set",
			old:  single(base()),
			new: func() *Document {
				d := single(base())
				d.Procedures[0].Deprecated = "use fetch"
				return d
			}(),
			want: Changes{{"procedure get", Added, "deprecated: use fetch"}},
		},
		{
			name: "no change",
			old:  single(base()),
			new:  single(base()),
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Diff(tc.old, tc.new)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got:\n%s\nwant:\n%s", FormatText(got), FormatText(tc.want))
			}
		})
	}
}

func TestDiffIgnoresRename(t *testing.T) {
	old := doc(map[string]*TypeDecl{"p.In": structDecl("In"), "p.User": structDecl("User", field("name", prim("string")))}, nil,
		query("get", ref("p.In"), ref("p.User")))
	renamed := doc(map[string]*TypeDecl{"q.Input": structDecl("Input"), "q.Person": structDecl("Person", field("name", prim("string")))}, nil,
		query("get", ref("q.Input"), ref("q.Person")))
	if got := Diff(old, renamed); len(got) != 0 {
		t.Fatalf("expected no changes, got %s", FormatText(got))
	}
}

func TestDiffFollowsGenericArgs(t *testing.T) {
	page := &TypeDecl{Kind: Generic, Name: "Page", Params: []string{"T"}, Body: &Type{Kind: Struct, Fields: []*Field{
		{Name: "items", Type: &Type{Kind: Array, Elem: &Type{Kind: Param, Name: "T"}}},
	}}}
	old := doc(map[string]*TypeDecl{"p.In": structDecl("In"), "p.Page": page, "p.User": structDecl("User", field("name", prim("string")))}, nil,
		query("list", ref("p.In"), ref("p.Page", ref("p.User"))))
	changed := doc(map[string]*TypeDecl{"p.In": structDecl("In"), "p.Page": page, "p.User": structDecl("User", field("name", prim("int64")))}, nil,
		query("list", ref("p.In"), ref("p.Page", ref("p.User"))))
	got := Diff(old, changed)
	want := Changes{{"procedure list output field items elem field name", Breaking, "type changed from string to int64"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %s", FormatText(got))
	}
}

func TestDiffHandlesRecursiveTypes(t *testing.T) {
	node := structDecl("Node", field("name", prim("string")), field("children", &Type{Kind: Array, Elem: ref("p.Node")}))
	old := doc(map[string]*TypeDecl{"p.In": structDecl("In"), "p.Node": node}, nil, query("get", ref("p.In"), ref("p.Node")))
	if got := Diff(old, old); len(got) != 0 {
		t.Fatalf("expected no changes, got %s", FormatText(got))
	}
}

func TestChangesBreaking(t *testing.T) {
	if (Changes{{"a", Widened, "x"}}).Breaking() {
		t.Fatal("widened is not breaking")
	}
	if !(Changes{{"a", Widened, "x"}, {"b", Breaking, "y"}}).Breaking() {
		t.Fatal("breaking not detected")
	}
}

func TestFormats(t *testing.T) {
	changes := Changes{
		{"procedure get input field id", Breaking, "required field added"},
		{"procedure get output field age", Added, "field added"},
	}
	text := FormatText(changes)
	if !strings.HasPrefix(text, "breaking  procedure get input field id: required field added\n") {
		t.Fatalf("text %q", text)
	}
	md := FormatMarkdown(changes)
	if !strings.HasPrefix(md, "**2 contract change(s), 1 breaking**\n\n- **breaking** `procedure get input field id`: required field added\n") {
		t.Fatalf("markdown %q", md)
	}
	if FormatText(nil) != "no contract changes\n" || FormatMarkdown(nil) != "no contract changes\n" {
		t.Fatal("empty formats")
	}
	data, err := FormatJSON(nil)
	if err != nil || string(data) != "[]\n" {
		t.Fatalf("empty json %q %v", data, err)
	}
	data, _ = FormatJSON(changes)
	var back Changes
	if err := json.Unmarshal(data, &back); err != nil || !reflect.DeepEqual(back, changes) {
		t.Fatalf("json round trip: %v %s", err, data)
	}
}
