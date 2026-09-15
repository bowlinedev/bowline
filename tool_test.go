package bowline

import (
	"strings"
	"testing"
)

func TestToolExposure(t *testing.T) {
	procs := NewRouter(
		Query("get", getUser, Tool(Scope("billing", "crm", "billing"))),
		Mutation("create", createUser, Tool(Scope("billing"), Destructive())),
		Query("hidden", getUser),
	).Procedures()
	get, create, hidden := procs[0], procs[1], procs[2]
	if !get.Exposed || !get.ReadOnly || get.Destructive || strings.Join(get.Scopes, ",") != "billing,crm" {
		t.Fatalf("get %+v", get)
	}
	if !create.Exposed || create.ReadOnly || !create.Destructive || strings.Join(create.Scopes, ",") != "billing" {
		t.Fatalf("create %+v", create)
	}
	if hidden.Exposed || hidden.ReadOnly || hidden.Scopes != nil {
		t.Fatalf("hidden %+v", hidden)
	}
}

func TestToolPanicsOnStreamsAndUploads(t *testing.T) {
	cases := map[string]func(){
		"subscription": func() { NewRouter(Subscription("watch", watch, Tool())) },
		"upload":       func() { NewRouter(Upload("attach", attach, Tool())) },
		"empty scope":  func() { NewRouter(Query("get", getUser, Tool(Scope("")))) },
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil || !strings.Contains(r.(string), name[:5]) && !strings.Contains(r.(string), "scope") {
					t.Fatalf("got %v", r)
				}
			}()
			fn()
		})
	}
}
