package goclient

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type node struct {
	segments []string
	children map[string]*node
	proc     *contract.Procedure
}

func buildTree(procs []*contract.Procedure) *node {
	root := &node{children: map[string]*node{}}
	for _, p := range procs {
		current := root
		segments := strings.Split(p.Path, ".")
		for i, s := range segments[:len(segments)-1] {
			child, ok := current.children[s]
			if !ok {
				child = &node{segments: segments[:i+1], children: map[string]*node{}}
				current.children[s] = child
			}
			current = child
		}
		current.children[segments[len(segments)-1]] = &node{segments: segments, proc: p}
	}
	return root
}

func (n *node) typeName() string {
	if len(n.segments) == 0 {
		return "Client"
	}
	return segmentsName(n.segments) + "Client"
}

func (n *node) receiver() string {
	if len(n.segments) == 0 {
		return "c *Client"
	}
	return "c " + n.typeName()
}

func (n *node) client() string {
	if len(n.segments) == 0 {
		return "c"
	}
	return "c.c"
}

func sortedKeys(n *node) []string {
	keys := slices.Sorted(maps.Keys(n.children))
	return keys
}

func (g *generator) client() string {
	root := buildTree(g.doc.Procedures)
	var b strings.Builder
	b.WriteString("// Client calls the API described by the contract.\n")
	b.WriteString("type Client struct {\n")
	g.writeMountFields(&b, root)
	b.WriteString("\n\tbase    string\n\thttp    *http.Client\n\theaders func(context.Context) (http.Header, error)\n}\n\n")
	b.WriteString(clientPrelude)
	b.WriteString("// New returns a client for the API served at baseURL.\n")
	b.WriteString("func New(baseURL string, opts ...Option) *Client {\n")
	b.WriteString("\tc := &Client{base: strings.TrimRight(baseURL, \"/\"), http: http.DefaultClient}\n")
	b.WriteString("\tfor _, opt := range opts {\n\t\topt(c)\n\t}\n")
	g.writeMountInit(&b, root, "c")
	b.WriteString("\treturn c\n}\n\n")
	g.writeMethods(&b, root)
	var subs []*node
	collectMounts(root, &subs)
	for _, sub := range subs {
		b.WriteString("// ")
		b.WriteString(sub.typeName())
		b.WriteString(" groups the procedures under ")
		b.WriteString(strconv.Quote(strings.Join(sub.segments, ".")))
		b.WriteString(".\n")
		b.WriteString("type ")
		b.WriteString(sub.typeName())
		b.WriteString(" struct {\n")
		g.writeMountFields(&b, sub)
		b.WriteString("\n\tc *Client\n}\n\n")
		g.writeMethods(&b, sub)
	}
	return b.String()
}

func collectMounts(n *node, out *[]*node) {
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			*out = append(*out, child)
			collectMounts(child, out)
		}
	}
}

func (g *generator) writeMountFields(b *strings.Builder, n *node) {
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			b.WriteString("\t")
			b.WriteString(exported(k))
			b.WriteString(" ")
			b.WriteString(child.typeName())
			b.WriteString("\n")
		}
	}
}

func (g *generator) writeMountInit(b *strings.Builder, n *node, expr string) {
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			field := expr + "." + exported(k)
			b.WriteString("\t")
			b.WriteString(field)
			b.WriteString(" = ")
			b.WriteString(child.typeName())
			b.WriteString("{c: c}\n")
			g.writeMountInit(b, child, field)
		}
	}
}

func (g *generator) writeMethods(b *strings.Builder, n *node) {
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			continue
		}
		p := child.proc
		name := exported(k)
		writeDoc(b, "", docOr(p.Doc, name+" calls "+p.Path+"."), p.Deprecated)
		switch p.Kind {
		case "subscription":
			g.writeSubscription(b, n, name, p)
		case "upload":
			g.writeUpload(b, n, name, p)
		default:
			g.writeCall(b, n, name, p)
		}
	}
}

func (g *generator) inputParam(p *contract.Procedure) (param, arg string) {
	if isEmptyStruct(p.Input) {
		return "", "struct{}{}"
	}
	return ", in " + g.goType(p.Input), "in"
}

func (g *generator) writeCall(b *strings.Builder, n *node, name string, p *contract.Procedure) {
	param, arg := g.inputParam(p)
	method := "http.MethodPost"
	if p.Method == "GET" {
		method = "http.MethodGet"
	}
	if isEmptyStruct(p.Output) {
		b.WriteString("func (")
		b.WriteString(n.receiver())
		b.WriteString(") ")
		b.WriteString(name)
		b.WriteString("(ctx context.Context")
		b.WriteString(param)
		b.WriteString(") error {\n")
		b.WriteString("\treturn ")
		b.WriteString(n.client())
		b.WriteString(".call(ctx, ")
		b.WriteString(strconv.Quote(p.Path))
		b.WriteString(", ")
		b.WriteString(method)
		b.WriteString(", ")
		b.WriteString(arg)
		b.WriteString(", nil)\n}\n\n")
		return
	}
	out := g.goType(p.Output)
	b.WriteString("func (")
	b.WriteString(n.receiver())
	b.WriteString(") ")
	b.WriteString(name)
	b.WriteString("(ctx context.Context")
	b.WriteString(param)
	b.WriteString(") (")
	b.WriteString(out)
	b.WriteString(", error) {\n")
	b.WriteString("\tvar out ")
	b.WriteString(out)
	b.WriteString("\n")
	b.WriteString("\terr := ")
	b.WriteString(n.client())
	b.WriteString(".call(ctx, ")
	b.WriteString(strconv.Quote(p.Path))
	b.WriteString(", ")
	b.WriteString(method)
	b.WriteString(", ")
	b.WriteString(arg)
	b.WriteString(", &out)\n")
	b.WriteString("\treturn out, err\n}\n\n")
}

func (g *generator) writeSubscription(b *strings.Builder, n *node, name string, p *contract.Procedure) {
	g.uses["iter"] = true
	param, arg := g.inputParam(p)
	method := "http.MethodPost"
	if p.Method == "GET" {
		method = "http.MethodGet"
	}
	out := g.goType(p.Output)
	b.WriteString("func (")
	b.WriteString(n.receiver())
	b.WriteString(") ")
	b.WriteString(name)
	b.WriteString("(ctx context.Context")
	b.WriteString(param)
	b.WriteString(") (iter.Seq2[")
	b.WriteString(out)
	b.WriteString(", error], error) {\n")
	b.WriteString("\tresp, err := ")
	b.WriteString(n.client())
	b.WriteString(".open(ctx, ")
	b.WriteString(strconv.Quote(p.Path))
	b.WriteString(", ")
	b.WriteString(method)
	b.WriteString(", ")
	b.WriteString(arg)
	b.WriteString(")\n")
	b.WriteString("\tif err != nil {\n\t\treturn nil, err\n\t}\n")
	b.WriteString("\treturn stream[")
	b.WriteString(out)
	b.WriteString("](resp, ")
	b.WriteString(strconv.Quote(p.Path))
	b.WriteString("), nil\n}\n\n")
}

func (g *generator) writeUpload(b *strings.Builder, n *node, name string, p *contract.Procedure) {
	g.uses["multipart"] = true
	param, arg := g.inputParam(p)
	if isEmptyStruct(p.Output) {
		b.WriteString("func (")
		b.WriteString(n.receiver())
		b.WriteString(") ")
		b.WriteString(name)
		b.WriteString("(ctx context.Context")
		b.WriteString(param)
		b.WriteString(", name string, file io.Reader) error {\n")
		b.WriteString("\treturn ")
		b.WriteString(n.client())
		b.WriteString(".upload(ctx, ")
		b.WriteString(strconv.Quote(p.Path))
		b.WriteString(", ")
		b.WriteString(arg)
		b.WriteString(", name, file, nil)\n}\n\n")
		return
	}
	out := g.goType(p.Output)
	b.WriteString("func (")
	b.WriteString(n.receiver())
	b.WriteString(") ")
	b.WriteString(name)
	b.WriteString("(ctx context.Context")
	b.WriteString(param)
	b.WriteString(", name string, file io.Reader) (")
	b.WriteString(out)
	b.WriteString(", error) {\n")
	b.WriteString("\tvar out ")
	b.WriteString(out)
	b.WriteString("\n")
	b.WriteString("\terr := ")
	b.WriteString(n.client())
	b.WriteString(".upload(ctx, ")
	b.WriteString(strconv.Quote(p.Path))
	b.WriteString(", ")
	b.WriteString(arg)
	b.WriteString(", name, file, &out)\n")
	b.WriteString("\treturn out, err\n}\n\n")
}

const clientPrelude = `// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the http.Client used for every call.
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) { cl.http = c }
}

// WithHeaders adds headers to every request, for example an Authorization header derived from ctx.
func WithHeaders(fn func(context.Context) (http.Header, error)) Option {
	return func(cl *Client) { cl.headers = fn }
}

`

const runtime = `// ErrorDetails carries a declared error variant's name and its details payload.
type ErrorDetails struct {
	Type string
	Raw  json.RawMessage
}

// VariantOf returns the declared error variant name carried by err, or "".
func VariantOf(err error) string {
	var be *bowline.Error
	if !errors.As(err, &be) {
		return ""
	}
	if d, ok := be.Details.(ErrorDetails); ok {
		return d.Type
	}
	return ""
}

// DetailsAs decodes the details payload of err into T when err is a Bowline error with details.
func DetailsAs[T any](err error) (T, bool) {
	var out T
	var be *bowline.Error
	if !errors.As(err, &be) {
		return out, false
	}
	d, ok := be.Details.(ErrorDetails)
	if !ok || len(d.Raw) == 0 {
		return out, false
	}
	if json.Unmarshal(d.Raw, &out) != nil {
		return out, false
	}
	return out, true
}

func (c *Client) request(ctx context.Context, path, method string, in any, accept string) (*http.Request, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return nil, &bowline.Error{Code: bowline.Internal, Message: fmt.Sprintf("encoding input for %s: %v", path, err)}
	}
	target := c.base + "/" + path
	var req *http.Request
	if method == http.MethodGet {
		target += "?input=" + url.QueryEscape(string(body))
		req, err = http.NewRequestWithContext(ctx, method, target, nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	}
	if err != nil {
		return nil, &bowline.Error{Code: bowline.Internal, Message: err.Error()}
	}
	if err := c.applyHeaders(ctx, req); err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) applyHeaders(ctx context.Context, req *http.Request) error {
	if c.headers == nil {
		return nil
	}
	h, err := c.headers(ctx)
	if err != nil {
		return err
	}
	for k, vs := range h {
		req.Header[k] = vs
	}
	return nil
}

func (c *Client) do(ctx context.Context, req *http.Request, path string) (*http.Response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &bowline.Error{Code: bowline.Canceled, Message: "request canceled"}
		}
		return nil, &bowline.Error{Code: bowline.Unavailable, Message: fmt.Sprintf("calling %s: %v", path, err)}
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, &bowline.Error{Code: bowline.Unavailable, Message: fmt.Sprintf("reading %s: %v", path, err)}
		}
		return nil, decodeError(resp.StatusCode, data, path)
	}
	return resp, nil
}

func (c *Client) call(ctx context.Context, path, method string, in, out any) error {
	req, err := c.request(ctx, path, method, in, "application/json")
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, req, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeBody(resp.Body, path, out)
}

func decodeBody(body io.Reader, path string, out any) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return &bowline.Error{Code: bowline.Unavailable, Message: fmt.Sprintf("reading %s: %v", path, err)}
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return &bowline.Error{Code: bowline.Internal, Message: fmt.Sprintf("decoding %s: %v", path, err)}
	}
	return nil
}

func decodeError(status int, data []byte, path string) error {
	var env struct {
		Error struct {
			Code    bowline.Code    ` + "`json:\"code\"`" + `
			Message string          ` + "`json:\"message\"`" + `
			Type    string          ` + "`json:\"type\"`" + `
			Details json.RawMessage ` + "`json:\"details\"`" + `
			Issues  []bowline.Issue ` + "`json:\"issues\"`" + `
		} ` + "`json:\"error\"`" + `
	}
	if err := json.Unmarshal(data, &env); err != nil || env.Error.Code == "" {
		return &bowline.Error{Code: bowline.Unknown, Message: fmt.Sprintf("HTTP %d from %s", status, path)}
	}
	e := &bowline.Error{Code: env.Error.Code, Message: env.Error.Message, Issues: env.Error.Issues}
	if env.Error.Type != "" || len(env.Error.Details) > 0 {
		e.Details = ErrorDetails{Type: env.Error.Type, Raw: env.Error.Details}
	}
	return e
}
`

const streamRuntime = `func (c *Client) open(ctx context.Context, path, method string, in any) (*http.Response, error) {
	req, err := c.request(ctx, path, method, in, "text/event-stream")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, req, path)
}

func stream[Out any](resp *http.Response, path string) iter.Seq2[Out, error] {
	return func(yield func(Out, error) bool) {
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64<<10), 16<<20)
		event, data := "", ""
		flush := func() bool {
			defer func() { event, data = "", "" }()
			switch event {
			case "message":
				var out Out
				if err := json.Unmarshal([]byte(data), &out); err != nil {
					var zero Out
					return yield(zero, &bowline.Error{Code: bowline.Internal, Message: fmt.Sprintf("decoding %s: %v", path, err)})
				}
				return yield(out, nil)
			case "error":
				var zero Out
				yield(zero, decodeError(http.StatusInternalServerError, []byte(data), path))
				return false
			case "done":
				return false
			}
			return true
		}
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case line == "":
				if event != "" && !flush() {
					return
				}
			case strings.HasPrefix(line, "event: "):
				event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data = strings.TrimPrefix(line, "data: ")
			}
		}
		if err := scanner.Err(); err != nil {
			var zero Out
			yield(zero, &bowline.Error{Code: bowline.Unavailable, Message: fmt.Sprintf("reading %s: %v", path, err)})
		}
	}
}
`

const uploadRuntime = `func (c *Client) upload(ctx context.Context, path string, in any, name string, file io.Reader, out any) error {
	input, err := json.Marshal(in)
	if err != nil {
		return &bowline.Error{Code: bowline.Internal, Message: fmt.Sprintf("encoding input for %s: %v", path, err)}
	}
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		part, err := writer.CreatePart(map[string][]string{"Content-Disposition": {` + "`form-data; name=\"input\"`" + `}, "Content-Type": {"application/json"}})
		if err == nil {
			_, err = part.Write(input)
		}
		if err == nil {
			var fw io.Writer
			fw, err = writer.CreateFormFile("file", name)
			if err == nil {
				_, err = io.Copy(fw, file)
			}
		}
		if err == nil {
			err = writer.Close()
		}
		pw.CloseWithError(err)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/"+path, pr)
	if err != nil {
		return &bowline.Error{Code: bowline.Internal, Message: err.Error()}
	}
	if err := c.applyHeaders(ctx, req); err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.do(ctx, req, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeBody(resp.Body, path, out)
}
`
