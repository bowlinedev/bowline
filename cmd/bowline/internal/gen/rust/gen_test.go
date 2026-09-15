package rust

import (
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/goldens"
	"github.com/bowlinedev/bowline/contract"
)

func TestGoldens(t *testing.T) {
	goldens.Run(t, "rust", "rs", Generator{})
}

func generate(t *testing.T, doc *contract.Document) string {
	t.Helper()
	if doc.Errors == nil {
		doc.Errors = map[string]*contract.ErrorDecl{}
	}
	out, err := Generator{}.Generate(doc, "src/bowline.rs")
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func prim(name string) *contract.Type {
	return &contract.Type{Kind: contract.Primitive, Name: name}
}

func ref(id string) *contract.Type {
	return &contract.Type{Kind: contract.Ref, ID: id}
}

func TestBoxOnInfiniteCycles(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"a.Node": {Kind: contract.Struct, Name: "Node", Fields: []*contract.Field{
			{Name: "parent", Type: ref("a.Node"), Optional: true},
			{Name: "children", Type: &contract.Type{Kind: contract.Array, Elem: ref("a.Node")}},
			{Name: "left", Type: ref("a.Leaf")},
		}},
		"a.Leaf": {Kind: contract.Struct, Name: "Leaf", Fields: []*contract.Field{
			{Name: "back", Type: ref("a.Node"), Nullable: true},
		}},
	}}
	out := generate(t, doc)
	for _, want := range []string{
		"pub parent: Option<Box<Node>>,",
		"pub children: Vec<Node>,",
		"pub left: Box<Leaf>,",
		"pub back: Option<Box<Node>>,",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestIntEnumImplsAndStringEnumRenames(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"a.Level":  {Kind: contract.Enum, Name: "Level", Base: "int32", Values: []contract.EnumValue{{Name: "LevelLow", Value: int64(1)}, {Name: "LevelHigh", Value: int64(10)}}},
		"a.Status": {Kind: contract.Enum, Name: "Status", Base: "string", Values: []contract.EnumValue{{Name: "StatusDraft", Value: "draft"}, {Name: "statusVoid", Value: "void"}}},
	}}
	out := generate(t, doc)
	for _, want := range []string{
		"impl Serialize for Level {",
		"serializer.serialize_i64(self.value())",
		"1 => Ok(Level::Low),",
		"10 => Ok(Level::High),",
		"#[serde(rename = \"draft\")]\n    Draft,",
		"#[serde(rename = \"void\")]\n    Void,",
		"Status::Void => \"void\",",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestCollectionsSnakeCaseAndEncodings(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"a.Grid": {Kind: contract.Struct, Name: "Grid", Fields: []*contract.Field{
			{Name: "byKey", Type: &contract.Type{Kind: contract.Map, Key: prim("string"), Value: prim("int32")}},
			{Name: "fixed", Type: &contract.Type{Kind: contract.Array, Elem: prim("int32"), Length: 3}},
			{Name: "type", Type: prim("string")},
			{Name: "big", Type: &contract.Type{Kind: contract.Primitive, Name: "int64", Encoding: "string"}},
			{Name: "maybeBig", Type: &contract.Type{Kind: contract.Primitive, Name: "uint64", Encoding: "string"}, Optional: true},
			{Name: "blob", Type: prim("bytes")},
			{Name: "blobs", Type: &contract.Type{Kind: contract.Array, Elem: prim("bytes")}},
			{Name: "bigs", Type: &contract.Type{Kind: contract.Array, Elem: &contract.Type{Kind: contract.Primitive, Name: "int64", Encoding: "string"}}},
		}},
	}}
	out := generate(t, doc)
	for _, want := range []string{
		"#[serde(rename = \"byKey\")]\n    pub by_key: std::collections::BTreeMap<String, i32>,",
		"pub fixed: [i32; 3],",
		"pub fixed: [i32; 3],\n    pub r#type: String,",
		"#[serde(with = \"bowline_client::codec::string_int\")]\n    pub big: i64,",
		"#[serde(rename = \"maybeBig\", default, skip_serializing_if = \"Option::is_none\", with = \"bowline_client::codec::string_uint_option\")]\n    pub maybe_big: Option<u64>,",
		"#[serde(with = \"bowline_client::codec::base64\")]\n    pub blob: Vec<u8>,",
		"pub blobs: Vec<bowline_client::Base64Bytes>,",
		"pub bigs: Vec<bowline_client::StringInt>,",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestClientShapeAndRules(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"a.In": {Kind: contract.Struct, Name: "In", Fields: []*contract.Field{
			{Name: "name", Type: prim("string"), Rules: []contract.Rule{{Rule: "required"}, {Rule: "max", Param: "80"}}},
			{Name: "age", Type: prim("int32"), Rules: []contract.Rule{{Rule: "min", Param: "18"}}},
			{Name: "nick", Type: prim("string"), Optional: true, Rules: []contract.Rule{{Rule: "min", Param: "2"}}},
			{Name: "kind", Type: prim("string"), Rules: []contract.Rule{{Rule: "oneof", Param: "a b"}}},
			{Name: "tags", Type: &contract.Type{Kind: contract.Array, Elem: prim("string")}, Rules: []contract.Rule{{Rule: "required"}, {Rule: "max", Param: "3"}}},
		}},
		"a.Out": {Kind: contract.Struct, Name: "Out", Fields: []*contract.Field{{Name: "ok", Type: prim("bool")}}},
	}, Procedures: []*contract.Procedure{
		{Path: "users.get", Kind: "query", Method: "GET", Input: ref("a.In"), Output: ref("a.Out")},
		{Path: "users.watch", Kind: "subscription", Method: "GET", Input: ref("a.In"), Output: ref("a.Out")},
		{Path: "users.attach", Kind: "upload", Method: "POST", Input: ref("a.In"), Output: ref("a.Out")},
		{Path: "ping", Kind: "query", Method: "GET", Input: &contract.Type{Kind: contract.Struct}, Output: &contract.Type{Kind: contract.Struct}},
	}}
	out := generate(t, doc)
	for _, want := range []string{
		"use bowline_client::{Body, CallOptions, Empty, Error, Issue, Method, Stream, Transport, Validate, rules};",
		"pub fn users(&self) -> UsersClient<'_> {",
		"pub async fn ping(&self, options: Option<&CallOptions>) -> Result<(), Error> {",
		".call(\"ping\", Method::Get, &Empty {}, options)",
		"pub struct UsersClient<'a> {",
		"pub async fn get(&self, input: &In, options: Option<&CallOptions>) -> Result<Out, Error> {",
		"-> impl Stream<Item = Result<Out, Error>> + Send + 'static {",
		"pub async fn attach(&self, input: &In, file: Body, filename: &str, options: Option<&CallOptions>) -> Result<Out, Error> {",
		"rules::required_str(&self.name, path, \"name\", issues);",
		"rules::max_len(rules::chars(&self.name), 80, \" characters\", path, \"name\", issues);",
		"rules::min_num(self.age as f64, 18.0, \"18\", path, \"age\", issues);",
		"if let Some(v) = &self.nick {\n            rules::min_len(rules::chars(v), 2, \" characters\", path, \"nick\", issues);\n        }",
		"rules::one_of(&self.kind, &[\"a\", \"b\"], path, \"kind\", issues);",
		"rules::required_len(self.tags.len(), path, \"tags\", issues);",
		"rules::max_len(self.tags.len(), 3, \" items\", path, \"tags\", issues);",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestRejectsAnUnrepresentableContract(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"example.com/app.Money": {Kind: contract.Kind("union"), Name: "Money"},
	}, Errors: map[string]*contract.ErrorDecl{}}
	_, err := Generator{}.Generate(doc, "bowline.out")
	if err == nil || !strings.Contains(err.Error(), "rust") || !strings.Contains(err.Error(), "example.com/app.Money") {
		t.Fatalf("got %v", err)
	}
}
